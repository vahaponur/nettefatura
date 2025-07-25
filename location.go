package nettefatura

import (
	_ "embed"
	"encoding/json"
	"strings"
)

//go:embed assets/il-ilce-data.json
var ilIlceDataJSON []byte

type IlIlceData struct {
	Cities    []City                `json:"cities"`
	Districts map[string][]District `json:"districts"`
}

type City struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type District struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

var locationData *IlIlceData

func init() {
	locationData = &IlIlceData{}
	if err := json.Unmarshal(ilIlceDataJSON, locationData); err != nil {
		panic("failed to load il-ilce data: " + err.Error())
	}
}

// normalizeString Türkçe karakterleri normalize eder ve küçük harfe çevirir
func normalizeString(s string) string {
	s = strings.ToLower(s)
	s = strings.TrimSpace(s)

	// Türkçe karakter dönüşümleri
	replacer := strings.NewReplacer(
		"ğ", "g",
		"ü", "u",
		"ş", "s",
		"ı", "i",
		"ö", "o",
		"ç", "c",
		"İ", "i",
		"I", "i",
		"Ğ", "g",
		"Ü", "u",
		"Ş", "s",
		"Ö", "o",
		"Ç", "c",
	)

	return replacer.Replace(s)
}

// GetCityID il adından il ID'sini bulur
func GetCityID(cityName string) string {
	normalized := normalizeString(cityName)

	for _, city := range locationData.Cities {
		if normalizeString(city.Name) == normalized {
			return city.ID
		}
	}

	return "-1"
}

// GetDistrictID il ID'si ve ilçe adından ilçe ID'sini bulur
func GetDistrictID(cityID, districtName string) int {
	districts, ok := locationData.Districts[cityID]
	if !ok {
		return -1
	}

	normalized := normalizeString(districtName)
	isLookingForMerkez := strings.Contains(districtName, normalizeString("Merkez"))
	// Önce direkt eşleşme dene
	for _, district := range districts {
		if strings.Contains(normalizeString(district.Name), "merkez") && isLookingForMerkez {
			return district.ID
		}
		if normalizeString(district.Name) == normalized {
			return district.ID
		}
	}

	// Eğer bulamazsa ve sadece il adı verilmişse merkez ilçeyi ara
	var cityName string
	for _, city := range locationData.Cities {
		if city.ID == cityID {
			cityName = city.Name
			break
		}
	}

	if cityName != "" {
		cityNameNormalized := normalizeString(cityName)

		// Sadece il adıyla tam eşleşiyorsa merkez ilçeleri ara
		if normalized == cityNameNormalized {
			// Önce il adını içeren merkez ilçeleri ara
			for _, district := range districts {
				districtNormalized := normalizeString(district.Name)
				if strings.Contains(districtNormalized, "merkez") &&
					strings.Contains(districtNormalized, cityNameNormalized) {
					return district.ID
				}
			}

			// Bulamazsa herhangi bir merkez ilçe
			for _, district := range districts {
				districtNormalized := normalizeString(district.Name)
				if strings.Contains(districtNormalized, "merkez") {
					return district.ID
				}
			}
		}
	}

	return -1
}

// GetDistrictIDByNames il adı ve ilçe adından direkt ilçe ID'sini bulur
func GetDistrictIDByNames(cityName, districtName string) int {
	cityID := GetCityID(cityName)
	if cityID == "-1" {
		return -1
	}

	return GetDistrictID(cityID, districtName)
}

// GetCityName il ID'sinden il adını bulur
func GetCityName(cityID string) string {
	for _, city := range locationData.Cities {
		if city.ID == cityID {
			return city.Name
		}
	}
	return "-1"
}

// GetDistrictName ilçe ID'sinden ilçe adını bulur
func GetDistrictName(cityID string, districtID int) string {
	districts, ok := locationData.Districts[cityID]
	if !ok {
		return "-1"
	}

	for _, district := range districts {
		if district.ID == districtID {
			return district.Name
		}
	}

	return "-1"
}
