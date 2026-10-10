package core

type AppSettings struct {
	ShowHiddenFiles     bool   `json:"showHiddenFiles"`
	TrashRetentionDays  int    `json:"trashRetentionDays"`
	RegistrationEnabled bool   `json:"registrationEnabled"`
	AppName             string `json:"appName"`
	LogoURL             string `json:"logoUrl"`
	FooterText          string `json:"footerText"`
	FileView            string `json:"fileView"`
	ShareExpiry         string `json:"shareExpiry"`
}
