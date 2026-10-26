package core

type SharedPageItem struct {
	Name, Kind, Size, DownloadURL string
	Folder                        bool
}

type SharedPageData struct {
	Branding                   AppSettings
	Name, Size, Kind, Download string
	Items                      []SharedPageItem
	Truncated                  bool
	RelockOnReturn             bool
}
