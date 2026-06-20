package customStructs

type CompanyProfile struct {
	InfoCompanyDirector        []Director        `json:"infoCompanyDirector"`
	ReqArticlePDF              []ArticlePDF      `json:"reqArticlePDF"`
	ReqComSumInfo              []CompanySummary  `json:"reqComSumInfo"`
	ReqTagsLogo                *TagLogo          `json:"reqTagsLogo"`
	ReqLogo                    *Logo             `json:"reqLogo"`
	InfoCompanyKeyExecutive    []KeyExecutive    `json:"infoCompanyKeyExecutive"`
	TopPosts                   []TopPost         `json:"topPosts"`
	ReqBlockAd                 []BlockAd         `json:"reqBlockAd"`
	InfoCompanyBusinessSummary []BusinessSummary `json:"infoCompanyBusinessSummary"`
}

type Director struct {
	DirectorID  int     `json:"directorId"`
	FirstName   string  `json:"firstName"`
	LastName    string  `json:"lastName"`
	Description *string `json:"description"` // Using pointer because it can be null
}

type ArticlePDF struct {
	ID         int     `json:"id"`
	Title      *string `json:"title"`
	Body       *string `json:"body"`
	SecurityID int     `json:"securityId"`
	ManualDate int64   `json:"manualDate"` // Unix millisecond timestamp
	AddedDate  int64   `json:"addedDate"`  // Unix millisecond timestamp
	Edited     int     `json:"edited"`
	Deleted    int     `json:"deleted"`
	FilePath   string  `json:"filePath"`
}

type CompanySummary struct {
	SecurityID       int     `json:"securityId"`
	RegisterOffice   string  `json:"registerOffice"`
	Tel1             string  `json:"tel1"`
	Name             string  `json:"name"`
	Symbol           string  `json:"symbol"`
	Fax              string  `json:"fax"`
	Email1           string  `json:"email1"`
	Web              string  `json:"web"`
	Established      string  `json:"established"`
	QuarterDate      string  `json:"quarterDate"` // Consider parsing this to time.Time later
	FinYearEnd       string  `json:"finYearEnd"`
	Capital          *string `json:"capital"`
	Authorized       *string `json:"authorized"`
	Issued           *string `json:"issued"`
	Secretaries      string  `json:"secretaries"`
	RegisteredOffice string  `json:"registeredOffice"`
	Auditors         string  `json:"auditors"`
	NatureOfBusiness *string `json:"natureOfbussinesId"`
	Sector           string  `json:"sector"`
	BoardType        string  `json:"boardType"`
}

type TagLogo struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type Logo struct {
	ID    int    `json:"id"`
	Path  string `json:"path"`
	SecID int    `json:"secId"`
}

type KeyExecutive struct {
}

type TopPost struct {
	DirectorID       int    `json:"directorId"`
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	DesignationOther string `json:"designationOther"`
}

type BlockAd struct {
	ID   int    `json:"id"`
	Path string `json:"path"`
}

type BusinessSummary struct {
	SummaryID int    `json:"summeryId"`
	Title     string `json:"title"`
	Body      string `json:"body"`
}
