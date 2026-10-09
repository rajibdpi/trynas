package core

type AppSettings struct {
	AppName     string `json:"appName"`
	FileView    string `json:"fileView"`
	ShareExpiry string `json:"shareExpiry"`
}
