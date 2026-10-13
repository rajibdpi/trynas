package core

type AppSettings struct {
	ThemeMode           string `json:"themeMode"`
	LightPrimary        string `json:"lightPrimary"`
	DarkPrimary         string `json:"darkPrimary"`
	LightBackground     string `json:"lightBackground"`
	DarkBackground      string `json:"darkBackground"`
	LightCard           string `json:"lightCard"`
	DarkCard            string `json:"darkCard"`
	ShowHiddenFiles     bool   `json:"showHiddenFiles"`
	TrashRetentionDays  int    `json:"trashRetentionDays"`
	RegistrationEnabled bool   `json:"registrationEnabled"`
	AppName             string `json:"appName"`
	LogoURL             string `json:"logoUrl"`
	FooterText          string `json:"footerText"`
	FileView            string `json:"fileView"`
	ShareExpiry         string `json:"shareExpiry"`
}
