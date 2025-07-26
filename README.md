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

#### İl/İlçe ID'leri ile:

```go
customer := nettefatura.Customer{
    Name:       "Ahmet Yılmaz",
    TaxNumber:  "11111111111", // TC Kimlik No
    Email:      "ahmet@example.com",
    Phone:      "5551234567",
    Address:    "Test Mahallesi Test Caddesi No:1",
    CityID:     "28",       // İstanbul
    CityName:   "İstanbul",
    DistrictID: "455",      // Kadıköy
    PostalCode: "34710",
}

customerID, err := client.CreateCustomer(customer)
if err != nil {
    log.Fatal(err)
}
```

#### İl/İlçe Helper Fonksiyonları ile:

```go
// İl ve ilçe isimlerinden ID bulma
cityID := nettefatura.GetCityID("İstanbul")        // "28"
districtID := nettefatura.GetDistrictID(cityID, "Kadıköy") // 455

// Veya direkt isimlerle
districtID := nettefatura.GetDistrictIDByNames("istanbul", "kadikoy") // 455

// Büyük/küçük harf ve Türkçe karakter duyarsız
cityID = nettefatura.GetCityID("istanbul")     // "28"
cityID = nettefatura.GetCityID("ISTANBUL")     // "28"
cityID = nettefatura.GetCityID("iStAnBuL")     // "28"

// Merkez ilçe desteği - il adı yazınca merkez ilçeyi bulur
districtID = nettefatura.GetDistrictIDByNames("Adıyaman", "Adıyaman") // Adıyaman Merkez

customer := nettefatura.Customer{
    Name:       "Ahmet Yılmaz",
    TaxNumber:  "11111111111",
    Email:      "ahmet@example.com",
    Phone:      "5551234567",
    Address:    "Test Mahallesi Test Caddesi No:1",
    CityID:     cityID,
    CityName:   nettefatura.GetCityName(cityID), // İsim almak için
    DistrictID: fmt.Sprintf("%d", districtID),
    PostalCode: "34710",
}

customerID, err := client.CreateCustomer(customer)
if err != nil {
    log.Fatal(err)
}
```

**Not:** İl ve ilçe ID'leri için `assets/il-ilce-data.json` dosyasına bakabilirsiniz.

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

#### Raw Response için CreateInvoiceRaw

Eğer ham response'a ihtiyacınız varsa (örneğin hata durumlarında bile 200 dönen API'ler için):

```go
rawResponse, err := client.CreateInvoiceRaw(invoice)
if err != nil {
    log.Fatal(err)
}

// Response'u kontrol et
responseStr := string(rawResponse)
if strings.Contains(responseStr, "error") || strings.Contains(responseStr, "html") {
    log.Printf("Fatura oluşturma hatası: %s", responseStr)
} else {
    // Başarılı response genelde sadece fatura numarasıdır
    invoiceNo := strings.Trim(responseStr, `"`)
    fmt.Printf("Fatura oluşturuldu: %s\n", invoiceNo)
}
```

### Müşteri ve Fatura Birlikte Oluşturma

```go
// Helper fonksiyonlarla il/ilçe bulma
cityID := nettefatura.GetCityID("Ankara")
districtID := nettefatura.GetDistrictIDByNames("Ankara", "Çankaya")

customer := &nettefatura.Customer{
    Name:       "Mehmet Demir",
    TaxNumber:  "11111111111",
    Email:      "mehmet@example.com",
    Phone:      "5559876543",
    CityID:     cityID,
    CityName:   "Ankara",
    DistrictID: fmt.Sprintf("%d", districtID),
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

### Müşteri Listesi ve Mevcut Müşteri Kontrolü

```go
// Müşteri listesini getir
recipientList, err := client.GetRecipientList(200) // Max 200 müşteri
if err != nil {
    log.Fatal(err)
}

fmt.Printf("Toplam müşteri: %d\n", recipientList.RecordsTotal)

// Müşteri detayı al
detail, err := client.GetRecipientDetail(recipientList.Data[0].IdAlici)
if err != nil {
    log.Fatal(err)
}

// Müşteri oluştur veya mevcut olanı bul
// Bu fonksiyon önce müşteri oluşturmayı dener
// Eğer "zaten kayıtlıdır" hatası alırsa:
// 1. Müşteri listesinden isim eşleşmesi arar
// 2. Birden fazla eşleşme varsa adres benzerliğine göre en uygununu seçer
customerID, err := client.CreateCustomerOrGetExisting(customer)
if err != nil {
    log.Fatal(err)
}
```

**Not:** Bireysel faturalarda TC kimlik numarası benzersiz değildir (11111111111 gibi). Bu yüzden birden fazla isim eşleşmesi olduğunda sistem adres, il ve ilçe benzerliğine göre skorlama yapar ve en uygun müşteriyi seçer.

### Tam Örnek - Kolay Fatura Oluşturma

```go
package main

import (
    "fmt"
    "log"
    "time"
    "github.com/vahaponur/nettefatura"
)

func main() {
    // Client oluştur
    client, err := nettefatura.NewClient("YOUR_COMPANY_ID")
    if err != nil {
        log.Fatal(err)
    }

    // Giriş yap
    err = client.Login("YOUR_VKN", "YOUR_PASSWORD")
    if err != nil {
        log.Fatal(err)
    }

    // İl/ilçe helper ile müşteri oluştur
    cityID := nettefatura.GetCityID("İzmir")
    districtID := nettefatura.GetDistrictID(cityID, "Karşıyaka")
    
    customer := &nettefatura.Customer{
        Name:       "Ali Veli",
        TaxNumber:  "11111111111",
        Email:      "ali@example.com",
        Phone:      "5551112233",
        Address:    "Karşıyaka Mahallesi No:1",
        CityID:     cityID,
        CityName:   "İzmir",
        DistrictID: fmt.Sprintf("%d", districtID),
        PostalCode: "35000",
    }

    // KDV dahil 1000 TL'lik fatura
    kdvHaric := nettefatura.CalculatePriceWithoutVAT(1000, 20)
    
    products := []nettefatura.Product{{
        Name:     "Davetiye Tasarım",
        Quantity: 1,
        Price:    kdvHaric,
        VATRate:  20,
    }}

    // Fatura oluştur
    invoiceNo, err := client.CreateInvoiceWithCustomer(customer, products)
    if err != nil {
        log.Fatal(err)
    }
    
    fmt.Printf("Fatura başarıyla oluşturuldu: %s\n", invoiceNo)
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

## İl/İlçe Helper Fonksiyonları

Paket, il ve ilçe ID'lerini kolayca bulmanız için helper fonksiyonlar içerir:

- `GetCityID(cityName string) string` - İl adından il ID'si bulur (bulamazsa "-1" döner)
- `GetDistrictID(cityID, districtName string) int` - İl ID'si ve ilçe adından ilçe ID'si bulur (bulamazsa -1 döner)
- `GetDistrictIDByNames(cityName, districtName string) int` - İl ve ilçe adlarından direkt ilçe ID'si bulur (bulamazsa -1 döner)
- `GetCityName(cityID string) string` - İl ID'sinden il adı bulur (bulamazsa "-1" döner)
- `GetDistrictName(cityID string, districtID int) string` - İlçe ID'sinden ilçe adı bulur (bulamazsa "-1" döner)

**Özellikler:**
- Büyük/küçük harf duyarsız (İstanbul = istanbul = ISTANBUL)
- Türkçe karakter duyarsız (Çanakkale = canakkale, Ağrı = agri)
- Merkez ilçe desteği (Adıyaman yazınca Adıyaman Merkez'i bulur)
- Tüm il/ilçe verileri `assets/il-ilce-data.json` dosyasında
- Bulunamayan il/ilçe durumunda `-1` döner

**Bulunamama Durumu Örneği:**
```go
cityID := nettefatura.GetCityID("YokBöyleBirİl")        // "-1"
districtID := nettefatura.GetDistrictID("28", "YokBöyleBirİlçe") // -1
districtID = nettefatura.GetDistrictIDByNames("YokBöyleBirİl", "Kadıköy") // -1
```

## Önemli Notlar

- Müşteriler bireysel veya kurumsal olarak oluşturulabilir
- TC kimlik no veya vergi numarası gereklidir
- Elektronik gönderim için e-posta adresi zorunludur
- Faturalar otomatik olarak onaylanır ve gönderilir

## Lisans

MIT