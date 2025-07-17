# NetteFatura Go Client

Go client paketi NetteFatura e-fatura sistemi için. Tarayıcı otomasyonu kullanarak API'yi tersine mühendislikle elde edilmiştir.

## Kurulum

```bash
go get github.com/vahaponur/nettefatura
```

## Kullanım

### KDV Hesaplama

Paket KDV HARİÇ fiyat ile çalışır. KDV dahil fiyattan hesaplama için yardımcı fonksiyonlar:

```go
// KDV dahil 1300 TL'lik fatura için:
kdvHaricFiyat := nettefatura.CalculatePriceWithoutVAT(1300, 20) // 1083.33 TL
kdvTutari := nettefatura.CalculateVATAmount(kdvHaricFiyat, 20)  // 216.67 TL

// Veya KDV hariç fiyattan KDV dahil hesaplama:
kdvDahilFiyat := nettefatura.CalculatePriceWithVAT(100, 20)     // 120 TL
```

### Client Oluşturma

```go
import "github.com/vahaponur/nettefatura"

// Zorunlu parametre: Company ID
client, err := nettefatura.NewClient("YOUR_COMPANY_ID_HERE")
if err != nil {
    log.Fatal(err)
}

// Opsiyonel konfigürasyon ile
client, err := nettefatura.NewClient("YOUR_COMPANY_ID_HERE",
    nettefatura.WithTimeout(60 * time.Second),
    nettefatura.WithCurrencyCode("USD"),
    nettefatura.WithMeasureUnit(100),
)
```

### Giriş Yapma

```go
err = client.Login("YOUR_VKN_HERE", "YOUR_PASSWORD_HERE")
if err != nil {
    log.Fatal(err)
}
```

### Müşteri Oluşturma

```go
customer := nettefatura.Customer{
    Name:       "Ahmet Yılmaz",
    TaxNumber:  "11111111111", // TC Kimlik No
    Email:      "ahmet@example.com",
    Phone:      "5551234567",
    Address:    "Test Mahallesi Test Caddesi No:1",
    CityID:     "28",       // İstanbul
    CityName:   "İstanbul",
    DistrictID: "413",      // Kadıköy
    PostalCode: "34710",
}

customerID, err := client.CreateCustomer(customer)
if err != nil {
    log.Fatal(err)
}
```

### Fatura Oluşturma

```go
// Örnek: KDV dahil 1300 TL'lik fatura
kdvHaricFiyat := nettefatura.CalculatePriceWithoutVAT(1300, 20) // 1083.33

products := []nettefatura.Product{
    {
        Name:     "Hizmet Bedeli",
        Quantity: 1,
        Price:    kdvHaricFiyat, // KDV hariç fiyat
        VATRate:  20,            // %20 KDV
    },
}

// Veya direkt KDV hariç fiyat ile:
products = []nettefatura.Product{
    {
        Name:     "Ürün 1",
        Quantity: 2,
        Price:    50.0, // KDV hariç birim fiyat
        VATRate:  20,   // %20 KDV
    },
    {
        Name:     "Ürün 2",
        Quantity: 1,
        Price:    30.0,
        VATRate:  20,
    },
}

invoice := nettefatura.Invoice{
    CustomerID: customerID,
    Products:   products,
    Date:       time.Now(),
    Notes:      []string{"Test faturası"},
}

invoiceNo, err := client.CreateInvoice(invoice)
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Fatura oluşturuldu: %s\n", invoiceNo)
```

### Müşteri ve Fatura Birlikte Oluşturma

```go
customer := &nettefatura.Customer{
    Name:       "Mehmet Demir",
    TaxNumber:  "11111111111",
    Email:      "mehmet@example.com",
    Phone:      "5559876543",
    CityID:     "28",
    CityName:   "İstanbul",
    DistrictID: "413",
}

products := []nettefatura.Product{
    {
        Name:     "Hizmet",
        Quantity: 1,
        Price:    100.0,
        VATRate:  20,
    },
}

invoiceNo, err := client.CreateInvoiceWithCustomer(customer, products)
if err != nil {
    log.Fatal(err)
}
```

## Konfigürasyon

### Environment Variables

Test için kullanılabilir:

```bash
export NETTEFATURA_VKN="YOUR_VKN_HERE"
export NETTEFATURA_PASSWORD="YOUR_PASSWORD_HERE"
export NETTEFATURA_COMPANY_ID="YOUR_COMPANY_ID_HERE"
```

### Client Options

- `WithBaseURL(url string)` - Custom base URL
- `WithTimeout(timeout time.Duration)` - HTTP client timeout
- `WithCurrencyCode(code string)` - Para birimi (varsayılan: TRY)
- `WithMeasureUnit(unit int)` - Ölçü birimi (varsayılan: 67 - Adet)

## Önemli Notlar

- Müşteriler bireysel veya kurumsal olarak oluşturulabilir
- TC kimlik no veya vergi numarası gereklidir
- Elektronik gönderim için e-posta adresi zorunludur
- Faturalar otomatik olarak onaylanır ve gönderilir

## Lisans

MIT