package core

type SharedPageItem struct {
	Name, Kind, Size, DownloadURL string
	Folder                        bool
}

type SharedPageData struct {
	Name, Size, Kind, Download string
	Items                      []SharedPageItem
	Truncated                  bool
}
