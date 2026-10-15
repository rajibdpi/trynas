package files

import (
	"archive/zip"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/rajibdpi/trynas/core"
)

func archiveFormat(name string) string {
	switch strings.ToLower(filepath.Ext(name)) {
	case ".zip":
		return "zip"
	case ".7z":
		return "7z"
	case ".xz":
		return "xz"
	case ".bz2", ".bzip2":
		return "bzip2"
	case ".gz", ".gzip":
		return "gzip"
	case ".tar":
		return "tar"
	case ".wim":
		return "wim"
	}
	return ""
}

func sevenZip() (string, error) {
	for _, name := range []string{"7zz", "7z"} {
		if binary, err := exec.LookPath(name); err == nil {
			return binary, nil
		}
	}
	return "", core.OperationError{Message: "Install 7-Zip (7zz or 7z) on the server to use this archive format", Status: 503}
}

type archiveEntry struct {
	Name               string
	UncompressedSize64 uint64
	mode               os.FileMode
	encrypted          bool
	open               func() (io.ReadCloser, error)
}

func (e archiveEntry) Open() (io.ReadCloser, error) { return e.open() }
func (e archiveEntry) Mode() os.FileMode            { return e.mode }
func (e archiveEntry) IsDir() bool                  { return e.mode.IsDir() }

type archiveReader struct {
	File  []archiveEntry
	close func() error
}

func (a *archiveReader) Close() error { return a.close() }

// The command never writes archive paths to disk. Entries are streamed through
// the same validation and bounded staging writer used for ZIP extraction.
func openArchive(ctx context.Context, source string) (*archiveReader, error) {
	if archiveFormat(source) == "zip" {
		z, err := zip.OpenReader(source)
		if err != nil {
			return nil, err
		}
		a := &archiveReader{close: z.Close}
		for _, f := range z.File {
			a.File = append(a.File, archiveEntry{Name: f.Name, UncompressedSize64: f.UncompressedSize64, mode: f.Mode(), encrypted: f.Flags&1 != 0, open: f.Open})
		}
		return a, nil
	}
	if archiveFormat(source) == "" {
		return nil, fmt.Errorf("unsupported archive format")
	}
	binary, err := sevenZip()
	if err != nil {
		return nil, err
	}
	requestCtx := ctx
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "l", "-slt", "-ba", "-sccUTF-8", "-p-", "--", source)
	var listing bytes.Buffer
	cmd.Stdout = &limitedArchiveWriter{writer: &listing, remaining: 8 << 20}
	if err = cmd.Run(); err != nil {
		return nil, fmt.Errorf("invalid, encrypted, or unsupported archive")
	}
	a := &archiveReader{close: func() error { return nil }}
	for _, block := range strings.Split(strings.ReplaceAll(listing.String(), "\r\n", "\n"), "\n\n") {
		fields := map[string]string{}
		for _, line := range strings.Split(strings.Trim(block, "\n"), "\n") {
			if line == "" {
				continue
			}
			key, value, ok := strings.Cut(line, " = ")
			if !ok || fields[key] != "" {
				return nil, fmt.Errorf("ambiguous archive metadata")
			}
			fields[key] = value
		}
		name, ok := fields["Path"]
		single := archiveFormat(source) == "xz" || archiveFormat(source) == "bzip2" || archiveFormat(source) == "gzip"
		if !ok && single && len(fields) > 0 {
			name = strings.TrimSuffix(filepath.Base(source), filepath.Ext(source))
			ok = true
		}
		if !ok {
			continue
		}
		if fields["Size"] == "" && single {
			measure := exec.CommandContext(ctx, binary, "x", "-so", "-p-", "--", source)
			counter := &limitedArchiveWriter{writer: io.Discard, remaining: 1 << 30}
			measure.Stdout = counter
			if err := measure.Run(); err != nil {
				return nil, fmt.Errorf("invalid archive or contents exceed 1 GiB")
			}
			fields["Size"] = strconv.FormatInt((1<<30)-counter.remaining, 10)
		}
		if fields["Folder"] == "+" && fields["Size"] == "" {
			fields["Size"] = "0"
		}
		size, err := strconv.ParseUint(fields["Size"], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid archive size")
		}
		mode := os.FileMode(0644)
		attrs := fields["Attributes"]
		if fields["Folder"] == "+" || strings.HasPrefix(attrs, "D") || strings.Contains(attrs, " d") {
			mode = os.ModeDir | 0755
		}
		if fields["Symbolic Link"] != "" || fields["Hard Link"] != "" || strings.Contains(attrs, " l") || strings.Contains(attrs, "L") {
			return nil, fmt.Errorf("archive links are unsupported")
		}
		entryName := name
		a.File = append(a.File, archiveEntry{Name: name, UncompressedSize64: size, mode: mode, encrypted: fields["Encrypted"] == "+", open: func() (io.ReadCloser, error) {
			streamCtx, stop := context.WithTimeout(requestCtx, 5*time.Minute)
			args := []string{"x", "-so", "-spd", "-p-"}
			if !single {
				args = append(args, "-i!"+entryName)
			}
			args = append(args, "--", source)
			command := exec.CommandContext(streamCtx, binary, args...)
			pipe, err := command.StdoutPipe()
			if err != nil {
				stop()
				return nil, err
			}
			if err = command.Start(); err != nil {
				stop()
				return nil, err
			}
			return &archiveStream{ReadCloser: pipe, command: command, stop: stop}, nil
		}})
		if len(a.File) > 10000 {
			return nil, core.OperationError{Message: "archive exceeds 10,000 entries", Status: 413}
		}
	}
	return a, nil
}

type archiveStream struct {
	io.ReadCloser
	command *exec.Cmd
	stop    context.CancelFunc
}

func (s *archiveStream) Close() error {
	s.ReadCloser.Close()
	err := s.command.Wait()
	s.stop()
	return err
}

type limitedArchiveWriter struct {
	writer    io.Writer
	remaining int64
}

func (w *limitedArchiveWriter) Write(p []byte) (int, error) {
	if int64(len(p)) > w.remaining {
		return 0, fmt.Errorf("archive output exceeds limit")
	}
	n, err := w.writer.Write(p)
	w.remaining -= int64(n)
	return n, err
}

// Convert a validated, locally generated ZIP using an isolated input tree.
func convertArchive(zipPath, format string) error {
	binary, err := sevenZip()
	if err != nil {
		return err
	}
	stage, err := os.MkdirTemp(filepath.Dir(zipPath), ".trynas-pack-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(stage)
	z, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	input := filepath.Join(stage, "input")
	if err = os.Mkdir(input, 0700); err != nil {
		z.Close()
		return err
	}
	files := []string{}
	for _, entry := range z.File {
		target := filepath.Join(input, filepath.FromSlash(entry.Name))
		if entry.FileInfo().IsDir() {
			err = os.MkdirAll(target, 0755)
		} else {
			err = os.MkdirAll(filepath.Dir(target), 0755)
			if err == nil {
				var in io.ReadCloser
				in, err = entry.Open()
				if err == nil {
					var out *os.File
					out, err = os.Create(target)
					if err == nil {
						_, err = io.Copy(out, in)
						closeErr := out.Close()
						if err == nil {
							err = closeErr
						}
					}
					in.Close()
				}
			}
			files = append(files, target)
		}
		if err != nil {
			z.Close()
			return err
		}
	}
	z.Close()
	source := "."
	if format == "xz" || format == "gzip" || format == "bzip2" {
		if len(files) != 1 || len(z.File) != 1 {
			return core.OperationError{Message: "XZ, BZIP2 and GZIP require a single file; use TAR for folders or multiple files", Status: 400}
		}
		source = files[0]
	}
	output := filepath.Join(stage, "archive."+format)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, "a", "-t"+format, "-y", "--", output, source)
	cmd.Dir = input
	if err = cmd.Run(); err != nil {
		return fmt.Errorf("could not create %s archive", format)
	}
	return os.Rename(output, zipPath)
}
