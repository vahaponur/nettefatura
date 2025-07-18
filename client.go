package nettefatura

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"strconv"
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
	products := make([]map[string]interface{}, 0, len(invoice.Products))
	var totalLineExtension, totalVAT, totalAmount float64

	for _, p := range invoice.Products {
		lineTotal := p.Price * p.Quantity
		vatAmount := lineTotal * float64(p.VATRate) / 100
		totalWithVAT := lineTotal + vatAmount

		totalLineExtension += lineTotal
		totalVAT += vatAmount
		totalAmount += totalWithVAT

		product := map[string]interface{}{
			"Name":               p.Name,
			"Quantity":           strconv.FormatFloat(p.Quantity, 'f', 2, 64),
			"UnitPrice":          strconv.FormatFloat(p.Price, 'f', 2, 64),
			"VatRate":            strconv.Itoa(p.VATRate),
			"IdMeasureUnit":      c.config.MeasureUnit,
			"LineExtensionAmount": strconv.FormatFloat(lineTotal, 'f', 2, 64),
			"VatAmount":          strconv.FormatFloat(vatAmount, 'f', 2, 64),
			"TaxInclusiveAmount": strconv.FormatFloat(totalWithVAT, 'f', 2, 64),
		}
		products = append(products, product)
	}

	// Not'ları birleştir
	notes := strings.Join(invoice.Notes, " ")

	// Fatura verisi hazırla
	invoiceData := map[string]interface{}{
		"IdCompany":                  c.config.CompanyID,
		"InvoiceProfileType":         "EARSIVFATURA",
		"IsQuickInvoice":             true,
		"InvoiceDate":                invoice.Date.Format("02.01.2006"),
		"InvoiceTime":                time.Now().Format("15:04:05"),
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
	
	// Müşteri bilgisi verilmişse önce müşteri oluştur
	if customer != nil {
		id, err := c.CreateCustomer(*customer)
		if err != nil {
			return "", fmt.Errorf("müşteri oluşturulamadı: %w", err)
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