package nettefatura

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Config client konfigürasyonu
type Config struct {
	BaseURL      string
	CompanyID    string
	MeasureUnit  int
	CurrencyCode string
	Timeout      time.Duration
}

// Option konfigürasyon fonksiyonu
type Option func(*Config)

// WithBaseURL custom base URL ayarlar
func WithBaseURL(url string) Option {
	return func(c *Config) {
		c.BaseURL = url
	}
}

// WithCompanyID firma ID'si ayarlar
func WithCompanyID(id string) Option {
	return func(c *Config) {
		c.CompanyID = id
	}
}

// WithMeasureUnit ölçü birimi ayarlar
func WithMeasureUnit(unit int) Option {
	return func(c *Config) {
		c.MeasureUnit = unit
	}
}

// WithCurrencyCode para birimi ayarlar
func WithCurrencyCode(code string) Option {
	return func(c *Config) {
		c.CurrencyCode = code
	}
}

// WithTimeout timeout ayarlar
func WithTimeout(timeout time.Duration) Option {
	return func(c *Config) {
		c.Timeout = timeout
	}
}

// Client NetteFatura API client
type Client struct {
	httpClient   *http.Client
	config       *Config
	token        string
}

// Customer müşteri bilgileri
type Customer struct {
	Name          string
	TaxNumber     string // TC Kimlik No
	Email         string
	Phone         string
	Address       string
	CityID        string
	CityName      string
	DistrictID    string
	PostalCode    string
	BuildingNo    string
	TaxOfficeID   string // Vergi dairesi ID (-1 for default)
	CustomerType  int    // 1=Bireysel, 2=Kurumsal
	SendingType   int    // 1=Elektronik, 2=Kağıt
}

// Product ürün bilgileri
type Product struct {
	Name     string
	Quantity float64
	Price    float64 // KDV hariç birim fiyat
	VATRate  int     // KDV oranı (%)
}

// Invoice fatura bilgileri
type Invoice struct {
	CustomerID string
	Products   []Product
	Date       time.Time
	Notes      []string
}

// RecipientListItem müşteri listesi öğesi
type RecipientListItem struct {
	IdAlici              int    `json:"IdAlici"`
	AliciAdi             string `json:"AliciAdi"`
	Vnktckn              string `json:"Vnktckn"`
	WebSite              string `json:"WebSite"`
	Telefon              string `json:"Telefon"`
	FaturaGonderimSekli  int    `json:"FaturaGonderimSekli"`
	IdIl                 int    `json:"IdIl"`
	IdIlce               int    `json:"IdIlce"`
	IlceAdi              string `json:"IlceAdi"`
	IlAdi                string `json:"IlAdi"`
	IdVergiDairesi       int    `json:"IdVergiDairesi"`
	Fax                  string `json:"Fax"`
	Email                string `json:"Email"`
	SokakAdi             string `json:"SokakAdi"`
	BinaNo               string `json:"BinaNo"`
	PostaKodu            string `json:"PostaKodu"`
	AliciTipi            int    `json:"AliciTipi"`
	IdAliciTipi          int    `json:"IdAliciTipi"`
	IdFirma              int    `json:"IdFirma"`
	IdAnaFirma           int    `json:"IdAnaFirma"`
	State                int    `json:"State"`
	IdAliciKaynak        string `json:"idAliciKaynak"`
	MusteriNo            string `json:"musteriNo"`
	IrsaliyeAlicisi      bool   `json:"IrsaliyeAlicisi"`
	StateName            string `json:"StateName"`
	MaskingRecipientName string `json:"MaskingRecipientName"`
}

// RecipientListResponse müşteri listesi API yanıtı
type RecipientListResponse struct {
	Draw            int                 `json:"draw"`
	RecordsTotal    int                 `json:"recordsTotal"`
	RecordsFiltered int                 `json:"recordsFiltered"`
	Data            []RecipientListItem `json:"data"`
}

// CalculatePriceWithoutVAT KDV dahil fiyattan KDV hariç fiyat hesaplar
func CalculatePriceWithoutVAT(priceWithVAT float64, vatRate int) float64 {
	return priceWithVAT / (1 + float64(vatRate)/100)
}

// CalculatePriceWithVAT KDV hariç fiyattan KDV dahil fiyat hesaplar
func CalculatePriceWithVAT(priceWithoutVAT float64, vatRate int) float64 {
	return priceWithoutVAT * (1 + float64(vatRate)/100)
}

// CalculateVATAmount KDV tutarını hesaplar
func CalculateVATAmount(priceWithoutVAT float64, vatRate int) float64 {
	return priceWithoutVAT * float64(vatRate) / 100
}

// NewClient yeni bir NetteFatura client oluşturur
func NewClient(companyID string, options ...Option) (*Client, error) {
	if companyID == "" {
		return nil, fmt.Errorf("company ID zorunludur")
	}

	// Default config
	config := &Config{
		BaseURL:      "https://nettefatura.isnet.net.tr",
		CompanyID:    companyID,
		MeasureUnit:  67, // Adet
		CurrencyCode: "TRY",
		Timeout:      30 * time.Second,
	}

	// Apply options
	for _, opt := range options {
		opt(config)
	}

	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("cookie jar oluşturulamadı: %w", err)
	}

	return &Client{
		httpClient: &http.Client{
			Jar:     jar,
			Timeout: config.Timeout,
		},
		config: config,
	}, nil
}

// Login sisteme giriş yapar
func (c *Client) Login(vknTckn, password string) error {
	// Token al
	if err := c.updateToken("/account/login"); err != nil {
		return fmt.Errorf("token alınamadı: %w", err)
	}

	// Login form
	form := url.Values{
		"VknTckn":                    {vknTckn},
		"Password":                   {password},
		"RememberMe":                 {"on"},
		"__RequestVerificationToken": {c.token},
	}

	resp, err := c.httpClient.PostForm(c.config.BaseURL+"/Account/Login", form)
	if err != nil {
		return fmt.Errorf("login isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	// 302 redirect veya 200 başarılı
	if resp.StatusCode != http.StatusFound && resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("login başarısız, status: %d, body: %s", resp.StatusCode, string(body))
	}

	return nil
}

// CreateCustomer yeni müşteri oluşturur
func (c *Client) CreateCustomer(customer Customer) (string, error) {
	// Token güncelle
	if err := c.updateToken("/Invoice/CreateQuick"); err != nil {
		return "", fmt.Errorf("token güncellenemedi: %w", err)
	}

	// Validasyonlar
	if customer.Name == "" {
		return "", fmt.Errorf("müşteri adı zorunludur")
	}
	if customer.TaxNumber == "" {
		return "", fmt.Errorf("TC kimlik no zorunludur")
	}
	if customer.SendingType == 1 && customer.Email == "" {
		return "", fmt.Errorf("elektronik gönderim için e-posta zorunludur")
	}

	// Varsayılan değerler
	if customer.CustomerType == 0 {
		customer.CustomerType = 1 // Bireysel
	}
	if customer.SendingType == 0 {
		customer.SendingType = 1 // Elektronik
	}
	if customer.TaxOfficeID == "" {
		customer.TaxOfficeID = "-1"
	}
	if customer.BuildingNo == "" {
		customer.BuildingNo = "1"
	}

	form := url.Values{
		"AliciAdi":                   {customer.Name},
		"Vnktckn":                    {customer.TaxNumber},
		"Email":                      {customer.Email},
		"Telefon":                    {customer.Phone},
		"FaturaGonderimSekli":        {fmt.Sprintf("%d", customer.SendingType)},
		"IdIl":                       {customer.CityID},
		"IdIlce":                     {customer.DistrictID},
		"IlAdi":                      {customer.CityName},
		"IdVergiDairesi":             {customer.TaxOfficeID},
		"SokakAdi":                   {customer.Address},
		"BinaNo":                     {customer.BuildingNo},
		"PostaKodu":                  {customer.PostalCode},
		"AliciTipi":                  {fmt.Sprintf("%d", customer.CustomerType)},
		"IdAliciTipi":                {"1"},
		"IdFirma":                    {c.config.CompanyID},
		"WebSite":                    {""},
		"Fax":                        {""},
		"Musterino":                  {""},
		"IrsaliyeAlicisi":            {"false"},
		"__RequestVerificationToken": {c.token},
	}

	req, err := http.NewRequest("POST", c.config.BaseURL+"/Recipient/Create", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("request oluşturulamadı: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("müşteri oluşturma isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("response okunamadı: %w", err)
	}

	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("JSON parse hatası: %w", err)
	}

	// Hata kontrolü
	if errorMsg, ok := result["error"].(string); ok && errorMsg != "" {
		return "", fmt.Errorf("müşteri oluşturma hatası: %s", errorMsg)
	}

	if errorMsg, ok := result["ErrorMessage"].(string); ok && errorMsg != "" {
		return "", fmt.Errorf("müşteri oluşturma hatası: %s", errorMsg)
	}

	// Başarılı - ID'yi al
	if idAlici, ok := result["IdAlici"].(float64); ok {
		return fmt.Sprintf("%.0f", idAlici), nil
	}

	return "", fmt.Errorf("müşteri ID bulunamadı: %s", string(body))
}

// CreateInvoice fatura oluşturur
func (c *Client) CreateInvoice(invoice Invoice) (string, error) {
	// Token güncelle
	if err := c.updateToken("/Invoice/CreateQuick"); err != nil {
		return "", fmt.Errorf("token güncellenemedi: %w", err)
	}

	// Fatura tarihi
	if invoice.Date.IsZero() {
		invoice.Date = time.Now()
	}

	// Ürünleri hazırla
	products := make([]map[string]interface{}, 0, len(invoice.Products))
	var totalLineExtension float64
	var totalVAT float64

	for _, product := range invoice.Products {
		lineTotal := product.Price * product.Quantity
		vatAmount := lineTotal * float64(product.VATRate) / 100
		
		totalLineExtension += lineTotal
		totalVAT += vatAmount

		products = append(products, map[string]interface{}{
			"ProductInvoiceModelId":   0,
			"DiscountAmount":          0,
			"DiscountRate":            0,
			"LineExtensionAmount":     lineTotal,
			"MeasureUnitId":           c.config.MeasureUnit,
			"ProductId":               nil,
			"ProductName":             product.Name,
			"Quantity":                product.Quantity,
			"UnitPrice":               product.Price,
			"VatAmount":               vatAmount,
			"VatRate":                 product.VATRate,
			"AdditionalTaxes":         []interface{}{},
			"WitholdingTaxes":         []interface{}{},
			"Deleted":                 false,
			"DeliveryList":            []interface{}{},
			"CustomsTrackingList":     []interface{}{},
			"TaxExemptionReason":      "",
			"TaxExemptionReasonCode":  "",
			"IdMensei":                0,
			"Mensei":                  nil,
			"SiniflandirmaKodu":       nil,
			"IdSiniflandirmaKodu":     0,
			"GTipNoArcvh":             "",
		})
	}

	totalAmount := totalLineExtension + totalVAT

	// Notes
	notes := invoice.Notes
	if len(notes) == 0 {
		notes = []string{""}
	}

	// Fatura JSON
	invoiceData := map[string]interface{}{
		"ETTN":                       "",
		"InvoiceId":                  "0",
		"RecipientType":              "2",
		"InvoiceNumber":              "",
		"CompanyId":                  c.config.CompanyID,
		"ScenarioType":               "0",
		"ReceiverInboxTag":           nil,
		"InvoiceDate":                invoice.Date.Format("02-01-2006"),
		"InvoiceTime":                invoice.Date.Format("15:04:05"),
		"InvoiceType":                "1", // Satış faturası
		"LastPaymentDate":            "",
		"DispatchList":               []interface{}{},
		"IdAlici":                    invoice.CustomerID,
		"Products":                   products,
		"CurrencyCode":               c.config.CurrencyCode,
		"CrossRate":                  0,
		"TaxExemptionReason":         "",
		"Notes":                      notes,
		"Receiver":                   map[string]string{"SendingType": "1"},
		"IsFreeOfCharge":             false,
		"KismiIadeMi":                false,
		"CompanyBankAccountList":     []interface{}{},
		"TotalLineExtensionAmount":   totalLineExtension,
		"TotalVATAmount":             totalVAT,
		"TotalTaxInclusiveAmount":    totalAmount,
		"TotalDiscountAmount":        0,
		"TotalPayableAmount":         totalAmount,
		"RoundCounter":               0,
	}

	jsonData, err := json.Marshal(invoiceData)
	if err != nil {
		return "", fmt.Errorf("JSON marshal hatası: %w", err)
	}

	form := url.Values{
		"jsonData":                   {string(jsonData)},
		"__RequestVerificationToken": {c.token},
	}

	req, err := http.NewRequest("POST", c.config.BaseURL+"/Invoice/Create", strings.NewReader(form.Encode()))
	if err != nil {
		return "", fmt.Errorf("request oluşturulamadı: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("fatura oluşturma isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("response okunamadı: %w", err)
	}

	// Başarılı response fatura numarasını string olarak döner
	invoiceNo := strings.Trim(string(body), `"`)
	if invoiceNo == "" || strings.Contains(invoiceNo, "error") {
		return "", fmt.Errorf("fatura oluşturulamadı: %s", string(body))
	}

	return invoiceNo, nil
}

// CreateInvoiceRaw creates invoice and returns raw response body
func (c *Client) CreateInvoiceRaw(invoice Invoice) ([]byte, error) {
	if invoice.CustomerID == "" {
		return nil, fmt.Errorf("müşteri ID gerekli")
	}

	// Token güncelle
	if err := c.updateToken("/Invoice/CreateQuick"); err != nil {
		return nil, fmt.Errorf("token güncellenemedi: %w", err)
	}

	// Ürünleri hazırla
	var products []map[string]interface{}
	var totalLineExtension, totalVAT float64

	for _, product := range invoice.Products {
		lineTotal := product.Price * product.Quantity
		vatAmount := lineTotal * float64(product.VATRate) / 100
		totalLineExtension += lineTotal
		totalVAT += vatAmount

		products = append(products, map[string]interface{}{
			"ProductInvoiceModelId":   0,
			"DiscountAmount":          0,
			"DiscountRate":            0,
			"LineExtensionAmount":     lineTotal,
			"MeasureUnitId":           c.config.MeasureUnit,
			"ProductId":               nil,
			"ProductName":             product.Name,
			"Quantity":                product.Quantity,
			"UnitPrice":               product.Price,
			"VatAmount":               vatAmount,
			"VatRate":                 product.VATRate,
			"AdditionalTaxes":         []interface{}{},
			"WitholdingTaxes":         []interface{}{},
			"Deleted":                 false,
			"DeliveryList":            []interface{}{},
			"CustomsTrackingList":     []interface{}{},
			"TaxExemptionReason":      "",
			"TaxExemptionReasonCode":  "",
			"IdMensei":                0,
			"Mensei":                  nil,
			"SiniflandirmaKodu":       nil,
			"IdSiniflandirmaKodu":     0,
			"GTipNoArcvh":             "",
		})
	}

	totalAmount := totalLineExtension + totalVAT

	// Notes
	notes := invoice.Notes
	if len(notes) == 0 {
		notes = []string{""}
	}

	// Fatura JSON - CreateInvoice ile aynı format
	invoiceData := map[string]interface{}{
		"ETTN":                       "",
		"InvoiceId":                  "0",
		"RecipientType":              "2",
		"InvoiceNumber":              "",
		"CompanyId":                  c.config.CompanyID,
		"ScenarioType":               "0",
		"ReceiverInboxTag":           nil,
		"InvoiceDate":                invoice.Date.Format("02-01-2006"),
		"InvoiceTime":                invoice.Date.Format("15:04:05"),
		"InvoiceType":                "1", // Satış faturası
		"LastPaymentDate":            "",
		"DispatchList":               []interface{}{},
		"IdAlici":                    invoice.CustomerID,
		"Products":                   products,
		"CurrencyCode":               c.config.CurrencyCode,
		"CrossRate":                  0,
		"TaxExemptionReason":         "",
		"Notes":                      notes,
		"Receiver":                   map[string]string{"SendingType": "1"},
		"IsFreeOfCharge":             false,
		"KismiIadeMi":                false,
		"CompanyBankAccountList":     []interface{}{},
		"TotalLineExtensionAmount":   totalLineExtension,
		"TotalVATAmount":             totalVAT,
		"TotalTaxInclusiveAmount":    totalAmount,
		"TotalDiscountAmount":        0,
		"TotalPayableAmount":         totalAmount,
		"RoundCounter":               0,
	}

	jsonData, err := json.Marshal(invoiceData)
	if err != nil {
		return nil, fmt.Errorf("JSON marshal hatası: %w", err)
	}

	form := url.Values{
		"jsonData":                   {string(jsonData)},
		"__RequestVerificationToken": {c.token},
	}

	req, err := http.NewRequest("POST", c.config.BaseURL+"/Invoice/Create", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fatura oluşturma isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response okunamadı: %w", err)
	}

	return body, nil
}

// CreateInvoiceWithCustomer müşteri yoksa oluşturur ve fatura keser
func (c *Client) CreateInvoiceWithCustomer(customer *Customer, products []Product) (string, error) {
	// Müşteri ID varsa direkt fatura oluştur
	customerID := ""
	
	// Müşteri bilgisi verilmişse önce müşteri oluştur veya mevcut olanı bul
	if customer != nil {
		id, err := c.CreateCustomerOrGetExisting(*customer)
		if err != nil {
			return "", fmt.Errorf("müşteri işlemi başarısız: %w", err)
		}
		customerID = id
	} else {
		return "", fmt.Errorf("müşteri bilgisi gerekli")
	}

	// Fatura oluştur
	invoice := Invoice{
		CustomerID: customerID,
		Products:   products,
		Date:       time.Now(),
	}

	invoiceNo, err := c.CreateInvoice(invoice)
	if err != nil {
		return "", fmt.Errorf("fatura oluşturulamadı: %w", err)
	}

	return invoiceNo, nil
}

// updateToken sayfadan CSRF token alır
func (c *Client) updateToken(path string) error {
	resp, err := c.httpClient.Get(c.config.BaseURL + path)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Token regex
	re := regexp.MustCompile(`name="__RequestVerificationToken".*?value="([^"]+)"`)
	matches := re.FindStringSubmatch(string(body))
	if len(matches) < 2 {
		return fmt.Errorf("token bulunamadı")
	}

	c.token = matches[1]
	return nil
}

// GetRecipientList müşteri listesini getirir
func (c *Client) GetRecipientList(limit int) (*RecipientListResponse, error) {
	// Default limit
	if limit <= 0 {
		limit = 200
	}

	// Form data for recipient list
	form := url.Values{
		"draw":             {"1"},
		"start":            {"0"},
		"length":           {fmt.Sprintf("%d", limit)},
		"search[value]":    {""},
		"search[regex]":    {"false"},
		"AliciTipi":        {"0"},
		"CompanyIdFilter":  {c.config.CompanyID},
		"RecipientState":   {"1"},
	}

	// Columns configuration
	columns := []struct {
		data       string
		searchable string
		orderable  string
	}{
		{"IdAlici", "false", "false"},
		{"ActionButtons", "true", "false"},
		{"AliciAdi", "true", "false"},
		{"Vnktckn", "true", "false"},
		{"StateName", "true", "false"},
		{"idAliciKaynak", "true", "false"},
	}

	// Add column parameters
	for i, col := range columns {
		form.Add(fmt.Sprintf("columns[%d][data]", i), col.data)
		form.Add(fmt.Sprintf("columns[%d][name]", i), "")
		form.Add(fmt.Sprintf("columns[%d][searchable]", i), col.searchable)
		form.Add(fmt.Sprintf("columns[%d][orderable]", i), col.orderable)
		form.Add(fmt.Sprintf("columns[%d][search][value]", i), "")
		form.Add(fmt.Sprintf("columns[%d][search][regex]", i), "false")
	}

	req, err := http.NewRequest("POST", c.config.BaseURL+"/Recipient/GetRecipientList", strings.NewReader(form.Encode()))
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("müşteri listesi isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response okunamadı: %w", err)
	}

	var result RecipientListResponse
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("JSON parse hatası: %w", err)
	}

	return &result, nil
}

// GetRecipientDetail müşteri detaylarını getirir
func (c *Client) GetRecipientDetail(recipientID int) (*Customer, error) {
	url := fmt.Sprintf("%s/Recipient/Detail?RecipientId=%d", c.config.BaseURL, recipientID)
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request oluşturulamadı: %w", err)
	}

	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("müşteri detay isteği başarısız: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("response okunamadı: %w", err)
	}

	// HTML parse - extract customer data
	htmlStr := string(body)
	customer := &Customer{}

	// Extract name
	if matches := regexp.MustCompile(`id="AliciAdi"\s+value="([^"]+)"`).FindStringSubmatch(htmlStr); len(matches) > 1 {
		customer.Name = strings.TrimSpace(matches[1])
	}

	// Extract tax number
	if matches := regexp.MustCompile(`id="VknTckn"\s+value="([^"]+)"`).FindStringSubmatch(htmlStr); len(matches) > 1 {
		customer.TaxNumber = matches[1]
	}

	// Extract email
	if matches := regexp.MustCompile(`id="Email"\s+value="([^"]+)"`).FindStringSubmatch(htmlStr); len(matches) > 1 {
		customer.Email = matches[1]
	}

	// Extract phone
	if matches := regexp.MustCompile(`id="Telefon"\s+value="([^"]+)"`).FindStringSubmatch(htmlStr); len(matches) > 1 {
		customer.Phone = matches[1]
	}

	// Extract address
	if matches := regexp.MustCompile(`id="SokakAdi"\s+value="([^"]+)"`).FindStringSubmatch(htmlStr); len(matches) > 1 {
		customer.Address = matches[1]
	}

	// Extract postal code
	if matches := regexp.MustCompile(`id="PostaKodu"\s+value="([^"]+)"`).FindStringSubmatch(htmlStr); len(matches) > 1 {
		customer.PostalCode = matches[1]
	}

	// Extract building no
	if matches := regexp.MustCompile(`id="BinaNo"\s+value="([^"]+)"`).FindStringSubmatch(htmlStr); len(matches) > 1 {
		customer.BuildingNo = matches[1]
	}

	// Extract city (selected option)
	if matches := regexp.MustCompile(`id="CityId".*?<option\s+value="(\d+)"\s+selected>([^<]+)</option>`).FindStringSubmatch(htmlStr); len(matches) > 2 {
		customer.CityID = matches[1]
		customer.CityName = strings.TrimSpace(matches[2])
	}

	// Extract district (would need another request as it's dynamically loaded)
	// For now, we'll leave district empty

	return customer, nil
}

// calculateSimilarityScore iki string arasındaki benzerlik skorunu hesaplar (0-1 arası)
func calculateSimilarityScore(s1, s2 string) float64 {
	// Normalize strings
	s1 = strings.ToLower(strings.TrimSpace(s1))
	s2 = strings.ToLower(strings.TrimSpace(s2))
	
	if s1 == s2 {
		return 1.0
	}
	
	if s1 == "" || s2 == "" {
		return 0.0
	}
	
	// Calculate Levenshtein distance
	longer := s1
	shorter := s2
	if len(s1) < len(s2) {
		longer = s2
		shorter = s1
	}
	
	longerLength := float64(len(longer))
	if longerLength == 0 {
		return 1.0
	}
	
	editDistance := levenshteinDistance(longer, shorter)
	return (longerLength - float64(editDistance)) / longerLength
}

// levenshteinDistance calculates the Levenshtein distance between two strings
func levenshteinDistance(s1, s2 string) int {
	if len(s1) == 0 {
		return len(s2)
	}
	if len(s2) == 0 {
		return len(s1)
	}
	
	// Create matrix
	matrix := make([][]int, len(s1)+1)
	for i := range matrix {
		matrix[i] = make([]int, len(s2)+1)
	}
	
	// Initialize first column and row
	for i := 0; i <= len(s1); i++ {
		matrix[i][0] = i
	}
	for j := 0; j <= len(s2); j++ {
		matrix[0][j] = j
	}
	
	// Fill matrix
	for i := 1; i <= len(s1); i++ {
		for j := 1; j <= len(s2); j++ {
			cost := 0
			if s1[i-1] != s2[j-1] {
				cost = 1
			}
			
			matrix[i][j] = min(
				matrix[i-1][j]+1,      // deletion
				matrix[i][j-1]+1,      // insertion
				matrix[i-1][j-1]+cost, // substitution
			)
		}
	}
	
	return matrix[len(s1)][len(s2)]
}

// min returns the minimum of three integers
func min(a, b, c int) int {
	if a < b {
		if a < c {
			return a
		}
		return c
	}
	if b < c {
		return b
	}
	return c
}

// parseIntOrZero parses string to int, returns 0 on error
func parseIntOrZero(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

// CreateCustomerOrGetExisting müşteri oluşturur veya mevcut müşteriyi döner
func (c *Client) CreateCustomerOrGetExisting(customer Customer) (string, error) {
	// Önce müşteri oluşturmayı dene
	customerID, err := c.CreateCustomer(customer)
	if err == nil {
		// Başarılı - yeni müşteri oluşturuldu
		return customerID, nil
	}

	// Hata mesajında "zaten kayıtlıdır" kontrolü
	if strings.Contains(err.Error(), "zaten kayıtlıdır") {
		// Müşteri zaten var - listeden bul
		recipientList, listErr := c.GetRecipientList(500) // Get more recipients to increase chance of finding
		if listErr != nil {
			return "", fmt.Errorf("müşteri listesi alınamadı: %w", listErr)
		}

		// İsme göre eşleşenleri bul
		var matches []RecipientListItem
		customerNameLower := strings.ToLower(strings.TrimSpace(customer.Name))
		
		for _, recipient := range recipientList.Data {
			recipientNameLower := strings.ToLower(strings.TrimSpace(recipient.AliciAdi))
			if recipientNameLower == customerNameLower {
				matches = append(matches, recipient)
			}
		}

		// Eşleşme bulunamadı
		if len(matches) == 0 {
			return "", fmt.Errorf("müşteri zaten kayıtlı ancak listede bulunamadı: %s", customer.Name)
		}

		// Tek eşleşme varsa direkt dön
		if len(matches) == 1 {
			return fmt.Sprintf("%d", matches[0].IdAlici), nil
		}

		// Birden fazla eşleşme var - adres benzerliğine göre sırala
		type scoredMatch struct {
			recipient RecipientListItem
			score     float64
		}
		
		var scoredMatches []scoredMatch
		
		for _, match := range matches {
			// Detaylı bilgi al
			detail, detailErr := c.GetRecipientDetail(match.IdAlici)
			if detailErr != nil {
				// Detay alınamazsa sadece mevcut bilgiyle skor hesapla
				score := 0.0
				
				// İl kontrolü
				if match.IdIl == parseIntOrZero(customer.CityID) {
					score += 0.3
				}
				
				// İlçe kontrolü
				if match.IdIlce == parseIntOrZero(customer.DistrictID) {
					score += 0.2
				}
				
				scoredMatches = append(scoredMatches, scoredMatch{
					recipient: match,
					score:     score,
				})
				continue
			}

			// Detaylı skorlama
			score := 0.0
			
			// Adres benzerliği (en önemli - %50)
			addressScore := calculateSimilarityScore(detail.Address, customer.Address)
			score += addressScore * 0.5
			
			// İl kontrolü (%30)
			if detail.CityID == customer.CityID {
				score += 0.3
			}
			
			// İlçe kontrolü (%20)
			if detail.DistrictID == customer.DistrictID {
				score += 0.2
			}
			
			scoredMatches = append(scoredMatches, scoredMatch{
				recipient: match,
				score:     score,
			})
		}
		
		// En yüksek skora sahip olanı bul
		if len(scoredMatches) > 0 {
			bestMatch := scoredMatches[0]
			for _, sm := range scoredMatches[1:] {
				if sm.score > bestMatch.score {
					bestMatch = sm
				}
			}
			return fmt.Sprintf("%d", bestMatch.recipient.IdAlici), nil
		}
		
		// Hiç skor hesaplanamadıysa ilk eşleşeni dön
		return fmt.Sprintf("%d", matches[0].IdAlici), nil
	}

	// Başka bir hata oluştu
	return "", err
}