package services

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/dhanavadh/fastfill-backend/internal/config"
)

type OCRService struct {
	config *config.Config
	client *http.Client
}

type ThaiIdCard struct {
	IDNumber         string  `json:"id_number"`
	IDNumberStatus   int     `json:"id_number_status"`
	NamePrefixTH     string  `json:"name_prefix_th"`
	ThName           string  `json:"th_name"`
	NamePrefixEN     string  `json:"name_prefix_en"`
	EnName           string  `json:"en_name"`
	Address          string  `json:"address"`
	BirthDate        string  `json:"birth_date"`
	BirthDateTH      string  `json:"birth_date_th"`
	IssueDate        string  `json:"issue_date"`
	IssueDateTH      string  `json:"issue_date_th"`
	ExpiryDate       string  `json:"expiry_date"`
	ExpiryDateTH     string  `json:"expiry_date_th"`
	Religion         string  `json:"religion"`
	LaserCode        string  `json:"laser_code"`
	DetectionScore   float64 `json:"detection_score"`
	RawText          string  `json:"raw_text"`
	ErrorMessage     string  `json:"error_message,omitempty"`
}

type VisionAPIRequest struct {
	Requests []VisionRequest `json:"requests"`
}

type VisionRequest struct {
	Image    Image                `json:"image"`
	Features []VisionAPIFeature   `json:"features"`
}

type Image struct {
	Content string `json:"content"`
}

type VisionAPIFeature struct {
	Type       string `json:"type"`
	MaxResults int    `json:"maxResults"`
}

type VisionAPIResponse struct {
	Responses []VisionResponse `json:"responses"`
}

type VisionResponse struct {
	FullTextAnnotation *FullTextAnnotation `json:"fullTextAnnotation,omitempty"`
	Error              *APIError           `json:"error,omitempty"`
}

type FullTextAnnotation struct {
	Text string `json:"text"`
}

type APIError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Status  string `json:"status"`
}

func NewOCRService(cfg *config.Config) *OCRService {
	return &OCRService{
		config: cfg,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (s *OCRService) ProcessThaiID(imageFile multipart.File) (*ThaiIdCard, error) {
	// Convert image to base64
	imageBytes, err := io.ReadAll(imageFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read image: %w", err)
	}

	base64Image := base64.StdEncoding.EncodeToString(imageBytes)

	// Call Google Vision API
	visionReq := VisionAPIRequest{
		Requests: []VisionRequest{
			{
				Image: Image{
					Content: base64Image,
				},
				Features: []VisionAPIFeature{
					{
						Type:       "DOCUMENT_TEXT_DETECTION",
						MaxResults: 1,
					},
				},
			},
		},
	}

	reqBody, err := json.Marshal(visionReq)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Get API key from config
	apiKey := s.config.GoogleVision.APIKey
	if apiKey == "" {
		return nil, fmt.Errorf("Google Vision API key not configured")
	}

	url := fmt.Sprintf("https://vision.googleapis.com/v1/images:annotate?key=%s", apiKey)
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to call Vision API: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("Vision API error: %d - %s", resp.StatusCode, string(body))
	}

	var visionResp VisionAPIResponse
	if err := json.NewDecoder(resp.Body).Decode(&visionResp); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if len(visionResp.Responses) == 0 {
		return nil, fmt.Errorf("no response from Vision API")
	}

	visionResponse := visionResp.Responses[0]
	if visionResponse.Error != nil {
		return nil, fmt.Errorf("Vision API error: %s", visionResponse.Error.Message)
	}

	if visionResponse.FullTextAnnotation == nil {
		return &ThaiIdCard{
			DetectionScore: 0,
			ErrorMessage:   "No text detected in image",
		}, nil
	}

	// Parse Thai ID card data
	return s.parseThaiIdCard(visionResponse.FullTextAnnotation.Text), nil
}

func (s *OCRService) parseThaiIdCard(fullText string) *ThaiIdCard {
	result := &ThaiIdCard{
		RawText: fullText,
	}

	normalizedText := s.normalizeThaiDigits(fullText)

	// Thai ID number pattern (13 digits)
	idPattern := regexp.MustCompile(`\b\d{1}[\s-]?\d{4}[\s-]?\d{5}[\s-]?\d{2}[\s-]?\d{1}\b|\b\d{13}\b`)
	if idMatch := idPattern.FindString(normalizedText); idMatch != "" {
		cleanId := regexp.MustCompile(`[\s-]`).ReplaceAllString(idMatch, "")
		result.IDNumber = cleanId
		if s.validateThaiId(cleanId) {
			result.IDNumberStatus = 1
		} else {
			result.IDNumberStatus = 0
		}
	}

	// Thai name with prefix
	thaiPrefixes := []string{"นาย", "นาง", "นางสาว", "เด็กหญิง", "เด็กชาย"}
	thaiPrefixPattern := strings.Join(thaiPrefixes, "|")
	thaiNamePattern := regexp.MustCompile(fmt.Sprintf(`(%s)\s+([ก-๏\s]+)`, thaiPrefixPattern))
	if thaiNameMatch := thaiNamePattern.FindStringSubmatch(fullText); len(thaiNameMatch) >= 3 {
		result.NamePrefixTH = strings.TrimSpace(thaiNameMatch[1])
		result.ThName = strings.TrimSpace(thaiNameMatch[2])
	}

	// English name with prefix
	englishPrefixes := []string{`Mr\.`, `Mrs\.`, `Miss`, `Ms\.`}
	englishPrefixPattern := strings.Join(englishPrefixes, "|")
	
	// Try multiline format first
	multilineEnglishPattern := regexp.MustCompile(fmt.Sprintf(`Name\s+(%s)\s+([A-Z][a-z]+)\s*\n?.*?Last name\s+([A-Z][a-z]+)`, englishPrefixPattern))
	if multilineMatch := multilineEnglishPattern.FindStringSubmatch(fullText); len(multilineMatch) >= 4 {
		result.NamePrefixEN = strings.ReplaceAll(strings.TrimSpace(multilineMatch[1]), ".", "")
		result.EnName = fmt.Sprintf("%s %s", strings.TrimSpace(multilineMatch[2]), strings.TrimSpace(multilineMatch[3]))
	} else {
		// Try single line pattern
		singleLinePattern := regexp.MustCompile(fmt.Sprintf(`(%s)\s+([A-Z][a-z]+(?:\s+[A-Z][a-z]+)*)`, englishPrefixPattern))
		if singleLineMatch := singleLinePattern.FindStringSubmatch(fullText); len(singleLineMatch) >= 3 {
			result.NamePrefixEN = strings.ReplaceAll(strings.TrimSpace(singleLineMatch[1]), ".", "")
			result.EnName = strings.TrimSpace(singleLineMatch[2])
		}
	}

	// Date patterns
	thaiDatePattern := regexp.MustCompile(`\d{1,2}\s+[ก-๏\.]+\s+\d{4}`)
	thaiDates := thaiDatePattern.FindAllString(normalizedText, -1)
	
	if len(thaiDates) > 0 {
		// Assign dates in order: birth, issue, expiry
		if len(thaiDates) >= 1 {
			result.BirthDateTH = thaiDates[0]
			result.BirthDate = s.convertBuddhistToGregorian(thaiDates[0])
		}
		if len(thaiDates) >= 2 {
			result.IssueDateTH = thaiDates[1]
			result.IssueDate = s.convertBuddhistToGregorian(thaiDates[1])
		}
		if len(thaiDates) >= 3 {
			result.ExpiryDateTH = thaiDates[2]
			result.ExpiryDate = s.convertBuddhistToGregorian(thaiDates[2])
		}
	}

	// Address pattern
	addressPattern := regexp.MustCompile(`(\d+\/?\d*\s+หมู่ที่?\s+\d+\s+ต\.[ก-๏\s]+อ\.[ก-๏\s]+จ\.[ก-๏\s]+)`)
	if addressMatch := addressPattern.FindString(normalizedText); addressMatch != "" {
		result.Address = regexp.MustCompile(`\s+`).ReplaceAllString(strings.TrimSpace(addressMatch), " ")
	}

	// Religion pattern
	religionPattern := regexp.MustCompile(`ศาสนา\s+([ก-๏]+)(?=\s|$|\n|ที่อยู่)`)
	if religionMatch := religionPattern.FindStringSubmatch(fullText); len(religionMatch) >= 2 {
		result.Religion = strings.TrimSpace(religionMatch[1])
	}

	// Laser code pattern
	laserCodePattern := regexp.MustCompile(`(\d{4}-\d{2}-\d{8}|\d{14})`)
	if laserCodeMatch := laserCodePattern.FindString(normalizedText); laserCodeMatch != "" {
		result.LaserCode = laserCodeMatch
	}

	// Calculate detection score
	score := 0.0
	if result.IDNumber != "" {
		score += 25.0
		if result.IDNumberStatus == 1 {
			score += 10.0
		}
	}
	if result.NamePrefixTH != "" { score += 5.0 }
	if result.ThName != "" { score += 15.0 }
	if result.NamePrefixEN != "" { score += 5.0 }
	if result.EnName != "" { score += 10.0 }
	if result.Address != "" { score += 10.0 }
	if result.BirthDate != "" { score += 5.0 }
	if result.BirthDateTH != "" { score += 5.0 }
	if result.IssueDate != "" { score += 5.0 }
	if result.ExpiryDate != "" { score += 5.0 }
	if result.Religion != "" { score += 5.0 }
	if result.LaserCode != "" { score += 5.0 }

	result.DetectionScore = score

	return result
}

func (s *OCRService) normalizeThaiDigits(text string) string {
	thaiDigits := "๐๑๒๓๔๕๖๗๘๙"
	arabicDigits := "0123456789"
	
	result := text
	for i, thaiDigit := range thaiDigits {
		arabicDigit := string(arabicDigits[i])
		result = strings.ReplaceAll(result, string(thaiDigit), arabicDigit)
	}
	return result
}

func (s *OCRService) validateThaiId(id string) bool {
	digits := regexp.MustCompile(`\D`).ReplaceAllString(id, "")
	if len(digits) != 13 {
		return false
	}
	
	sum := 0
	for i := 0; i < 12; i++ {
		digit, _ := strconv.Atoi(string(digits[i]))
		sum += digit * (13 - i)
	}
	
	check := (11 - (sum % 11)) % 10
	lastDigit, _ := strconv.Atoi(string(digits[12]))
	return check == lastDigit
}

var thaiMonths = map[string]string{
	"ม.ค.": "Jan", "มกราคม": "Jan", "มค": "Jan",
	"ก.พ.": "Feb", "กุมภาพันธ์": "Feb", "กพ": "Feb",
	"มี.ค.": "Mar", "มีนาคม": "Mar", "มีค": "Mar",
	"เม.ย.": "Apr", "เมษายน": "Apr", "เมย": "Apr",
	"พ.ค.": "May", "พฤษภาคม": "May", "พค": "May",
	"มิ.ย.": "Jun", "มิถุนายน": "Jun", "มิย": "Jun",
	"ก.ค.": "Jul", "กรกฎาคม": "Jul", "กค": "Jul",
	"ส.ค.": "Aug", "สิงหาคม": "Aug", "สค": "Aug",
	"ก.ย.": "Sep", "กันยายน": "Sep", "กย": "Sep",
	"ต.ค.": "Oct", "ตุลาคม": "Oct", "ตค": "Oct",
	"พ.ย.": "Nov", "พฤศจิกายน": "Nov", "พย": "Nov",
	"ธ.ค.": "Dec", "ธันวาคม": "Dec", "ธค": "Dec",
}

func (s *OCRService) convertBuddhistToGregorian(thaiDate string) string {
	// Match Thai date format: DD MMM YYYY
	datePattern := regexp.MustCompile(`(\d{1,2})\s+([ก-๏\.]+)\s+(\d{4})`)
	dateMatch := datePattern.FindStringSubmatch(thaiDate)
	if len(dateMatch) < 4 {
		return thaiDate
	}

	day := dateMatch[1]
	thaiMonth := strings.TrimSpace(dateMatch[2])
	year := dateMatch[3]

	monthEn := thaiMonths[thaiMonth]
	if monthEn == "" {
		monthEn = thaiMonth
	}

	// Convert Buddhist Era to Gregorian (BE - 543 = CE)
	yearInt, err := strconv.Atoi(year)
	if err != nil {
		return thaiDate
	}
	
	if yearInt >= 2400 {
		yearInt -= 543
	}

	return fmt.Sprintf("%s %s %d", day, monthEn, yearInt)
}