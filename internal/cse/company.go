package cse

import (
	"context"
	"encoding/json"
	"net/url"
)

// CompanyProfile is the companyProfile response.
type CompanyProfile struct {
	Directors       []Director        `json:"infoCompanyDirector"`
	Articles        []ArticlePDF      `json:"reqArticlePDF"`
	Summary         []CompanySummary  `json:"reqComSumInfo"`
	TagsLogo        *Logo             `json:"reqTagsLogo"`
	Logo            *Logo             `json:"reqLogo"`
	KeyExecutives   []json.RawMessage `json:"infoCompanyKeyExecutive"` // shape not yet observed
	TopPosts        []TopPost         `json:"topPosts"`
	BlockAds        []Asset           `json:"reqBlockAd"`
	BusinessSummary []BusinessSummary `json:"infoCompanyBusinessSummary"`
}

// Director is a board member. FirstName often includes the role in brackets.
type Director struct {
	DirectorID  int     `json:"directorId"`
	FirstName   string  `json:"firstName"`
	LastName    string  `json:"lastName"`
	Description *string `json:"description"`
}

// ArticlePDF is a document attached to the company profile.
type ArticlePDF struct {
	ID         int     `json:"id"`
	Title      *string `json:"title"`
	Body       *string `json:"body"`
	SecurityID int     `json:"securityId"`
	ManualDate int64   `json:"manualDate"` // ms
	AddedDate  int64   `json:"addedDate"`  // ms
	Edited     int     `json:"edited"`
	Deleted    int     `json:"deleted"`
	FilePath   string  `json:"filePath"`
}

// CompanySummary holds the registration details, sector and board.
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
	QuarterDate      string  `json:"quarterDate"`
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

// Logo is an image owned by a security.
type Logo struct {
	ID    int    `json:"id"`
	Path  string `json:"path"`
	SecID int    `json:"secId"`
}

// Asset is a bare id + path pair (banner and block ads).
type Asset struct {
	ID   int64  `json:"id"`
	Path string `json:"path"`
}

// TopPost is a key officer (chairman, CEO…).
type TopPost struct {
	DirectorID       int    `json:"directorId"`
	FirstName        string `json:"firstName"`
	LastName         string `json:"lastName"`
	DesignationOther string `json:"designationOther"`
}

// BusinessSummary is a free-text section about the business.
type BusinessSummary struct {
	SummaryID int    `json:"summeryId"`
	Title     string `json:"title"`
	Body      string `json:"body"`
}

// CompanyProfile fetches the profile for a security code (or ticker).
func (c *Client) CompanyProfile(ctx context.Context, code string) (*CompanyProfile, error) {
	var p CompanyProfile
	if err := c.post(ctx, "companyProfile", url.Values{"symbol": {code}}, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// Report is one uploaded financial report PDF.
type Report struct {
	ID             int64   `json:"id"`
	Path           string  `json:"path"`
	Path2          *string `json:"path2"`
	ManualDate     int64   `json:"manualDate"`     // ms; the period the report is for
	UploadedDate   int64   `json:"uploadedDate"`   // ms
	AuthorizedDate *int64  `json:"authorizedDate"` // ms; null when not yet authorised
	FileText       string  `json:"fileText"`
}

// ReqFinancial is an extra financial data element from the financials page.
type ReqFinancial struct {
	SecID int64  `json:"secId"`
	ElmID string `json:"elmId"`
	Data  string `json:"data"`
}

// Financials is the financials response: links to report PDFs.
type Financials struct {
	Annual    []Report       `json:"infoAnnualData"`
	Quarterly []Report       `json:"infoQuarterlyData"`
	Other     []Report       `json:"infoOtherData"`
	Extra     []ReqFinancial `json:"reqFinancial"`
	BannerAd  Asset          `json:"infoCompanyBannerAd"`
	WebLinks  []*string      `json:"infoWebLink"`
}

// Financials fetches the report list for a security code (or ticker).
func (c *Client) Financials(ctx context.Context, code string) (*Financials, error) {
	var f Financials
	if err := c.post(ctx, "financials", url.Values{"symbol": {code}}, &f); err != nil {
		return nil, err
	}
	return &f, nil
}
