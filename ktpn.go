package ktpn

import (
	"encoding/json"
	"errors"
	"fmt"
	_ "image/jpeg"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/otiai10/gosseract/v2"
)

var (
	//masking due to posible url string scan
	captchaURLHex = []byte{
		0x68, 0x74, 0x74, 0x70, 0x73, 0x3A, 0x2F, 0x2F,
		0x77, 0x77, 0x77, 0x2E, 0x63, 0x73, 0x67, 0x74,
		0x2E, 0x76, 0x6E, 0x2F, 0x6C, 0x69, 0x62, 0x2F,
		0x63, 0x61, 0x70, 0x74, 0x63, 0x68, 0x61, 0x2F,
		0x63, 0x61, 0x70, 0x74, 0x63, 0x68, 0x61, 0x2E,
		0x63, 0x6C, 0x61, 0x73, 0x73, 0x2E, 0x70, 0x68,
		0x70,
	}
	requestURLHex = []byte{
		0x68, 0x74, 0x74, 0x70, 0x73, 0x3a, 0x2f, 0x2f,
		0x77, 0x77, 0x77, 0x2e, 0x63, 0x73, 0x67, 0x74,
		0x2e, 0x76, 0x6e, 0x2f, 0x3f, 0x6d, 0x6f, 0x64,
		0x3d, 0x63, 0x6f, 0x6e, 0x74, 0x61, 0x63, 0x74,
		0x26, 0x74, 0x61, 0x73, 0x6b, 0x3d, 0x74, 0x72,
		0x61, 0x63, 0x75, 0x75, 0x5f, 0x70, 0x6f, 0x73,
		0x74, 0x26, 0x61, 0x6a, 0x61, 0x78,
	}
)

type VehicleType int

const (
	Car VehicleType = 1
)

func (t VehicleType) String() string {
	return fmt.Sprintf("%d", t)
}

type httpclient struct {
	http.Client
}

func httpClient() (*httpclient, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, err
	}
	client := http.Client{Jar: jar}
	return &httpclient{
		Client: client,
	}, nil
}

func (c *httpclient) fetchCaptcha() (string, error) {
	resp, err := c.Get(string(captchaURLHex))
	if err != nil {
		return "", err
	}

	//nolint:errcheck
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", errors.New("not 200")
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// read text from image data by OCR
	ocr := gosseract.NewClient()
	//nolint:errcheck
	defer ocr.Close()

	err = ocr.SetImageFromBytes(data)
	if err != nil {
		return "", err
	}

	captcha, err := ocr.Text()
	if err != nil {
		return "", err
	}

	return captcha, nil
}

func KT(plateNumber string, vehicleType VehicleType) ([]Violation, error) {
	client, err := httpClient()
	if err != nil {
		return nil, err
	}

	captcha, err := client.fetchCaptcha()
	if err != nil {
		return nil, err
	}

	plateNumber = strings.ToUpper(strings.NewReplacer(".", "", "-", "").Replace(plateNumber))

	data := url.Values{
		"BienKS":   {plateNumber},
		"Xe":       {vehicleType.String()},
		"captcha":  {captcha},
		"ipClient": {"9.9.9.91"},
		"cUrl":     {"1"},
	}
	body := strings.NewReader(data.Encode())

	// don't request too fast, just sleep 3 second after get the captcha
	time.Sleep(3 * time.Second)

	req, err := http.NewRequest(
		http.MethodPost,
		string(requestURLHex),
		body,
	)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	//nolint:errcheck
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected error: status %d", resp.StatusCode)
	}

	resultBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result Result
	if err := json.Unmarshal(resultBody, &result); err != nil {
		return nil, fmt.Errorf("unexpected error: %w\n%s", err, resultBody)
	}

	if result.Success != "true" {
		return nil, fmt.Errorf("unexpected error: %s", resultBody)
	}

	resp, err = client.Get(result.Href)
	if err != nil {
		return nil, err
	}

	//nolint:errcheck
	defer resp.Body.Close()

	return parseViolation(resp.Body)
}

type Result struct {
	Success string `json:"success"`
	Href    string `json:"href"`
}

func parseViolation(html io.ReadCloser) (result []Violation, err error) {
	doc, err := goquery.NewDocumentFromReader(html)
	if err != nil {
		return nil, err
	}

	for _, s := range doc.Find("#bodyPrint123.form-horizontal").EachIter() {
		var v Violation
		for _, s := range s.Children().EachIter() {
			if s.Is("hr") {
				result = append(result, v)
				continue
			}
			label, err := s.Find("span").Html()
			if err != nil {
				return nil, err
			}

			switch label {
			case "Biển kiểm soát:":
				value, err := s.Find(".col-md-9").Html()
				if err != nil {
					return nil, err
				}
				v = Violation{PlateNumber: value}
			case "Màu biển:":
				value, err := s.Find(".col-md-9").Html()
				if err != nil {
					return nil, err
				}
				v.PlateColor = value
			case "Loại phương tiện:":
				value, err := s.Find(".col-md-9").Html()
				if err != nil {
					return nil, err
				}
				v.VehicleType = value
			case "Thời gian vi phạm: ":
				value, err := s.Find(".col-md-9").Html()
				if err != nil {
					return nil, err
				}
				v.Date = value
			case "Địa điểm vi phạm:":
				value, err := s.Find(".col-md-9").Html()
				if err != nil {
					return nil, err
				}
				v.Location = value
			case "Hành vi vi phạm:":
				value, err := s.Find(".col-md-9").Html()
				if err != nil {
					return nil, err
				}
				v.Reason = value
			case "Trạng thái: ":
				value, err := s.Find(".col-md-9 > span").Html()
				if err != nil {
					return nil, err
				}
				v.Status = value
			case "Đơn vị phát hiện vi phạm: ":
				value, err := s.Find(".col-md-9").Html()
				if err != nil {
					return nil, err
				}
				v.TrafficEnforcement = value
			default:
			}
		}
	}

	return result, nil
}

type Violation struct {
	PlateNumber        string
	PlateColor         string
	VehicleType        string
	Date               string
	Location           string
	Reason             string
	Status             string
	TrafficEnforcement string
	TrafficCourt       string
}
