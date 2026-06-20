package customStructs

type InfoAnnualData struct {
	ID             int64   `json:"id"`
	Path           string  `json:"path"`
	ManualDate     int64   `json:"manualDate"`
	UploadedDate   int64   `json:"uploadedDate"`
	FileText       string  `json:"fileText"`
	Path2          *string `json:"path2"` // Pointer to handle null
	AuthorizedDate int64   `json:"authorizedDate"`
}

type InfoQuarterlyData struct {
	ID             int64   `json:"id"`
	Path           string  `json:"path"`
	ManualDate     int64   `json:"manualDate"`
	UploadedDate   int64   `json:"uploadedDate"`
	FileText       string  `json:"fileText"`
	Path2          *string `json:"path2"`
	AuthorizedDate int64   `json:"authorizedDate"`
}

type InfoOtherData struct {
	ID             int64   `json:"id"`
	Path           string  `json:"path"`
	ManualDate     int64   `json:"manualDate"`
	UploadedDate   int64   `json:"uploadedDate"`
	FileText       string  `json:"fileText"`
	Path2          *string `json:"path2"`
	AuthorizedDate *int64  `json:"authorizedDate"`
}

type ReqFinancial struct {
	SecID int64  `json:"secId"`
	ElmID string `json:"elmId"`
	Data  string `json:"data"`
}

type InfoCompanyBannerAd struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

// Wrapper Struct
type FinancialDataPayload struct {
	InfoAnnualData      []InfoAnnualData    `json:"infoAnnualData"`
	InfoQuarterlyData   []InfoQuarterlyData `json:"infoQuarterlyData"`
	InfoOtherData       []InfoOtherData     `json:"infoOtherData"`
	ReqFinancial        []ReqFinancial      `json:"reqFinancial"`
	InfoCompanyBannerAd InfoCompanyBannerAd `json:"infoCompanyBannerAd"`
	InfoWebLink         []*string           `json:"infoWebLink,omitempty"`
}
