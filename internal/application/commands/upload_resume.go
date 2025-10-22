package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	dbs "github.com/Builder-Lawyers/builder-backend/pkg/db"
	"github.com/ledongthuc/pdf"
)

type UploadResume struct {
	*dbs.UOWFactory
}

func NewUploadResume(factory *dbs.UOWFactory) *UploadResume {
	return &UploadResume{UOWFactory: factory}
}

type UserInfo struct {
	Name        string `json:"Name,omitempty"`
	Surname     string `json:"Surname,omitempty"`
	Summary     string `json:"Summary,omitempty"`
	Experiences []struct {
		Company                   string   `json:"Company,omitempty"`
		TwoMostUsefulAchievements []string `json:"TwoMostUsefulAchievements,omitempty"`
	} `json:"Experiences,omitempty"`
	Skills   []string `json:"Skills,omitempty"`
	Contacts struct {
		Mail    string `json:"Mail,omitempty"`
		Phone   string `json:"Phone,omitempty"`
		Address string `json:"Address,omitempty"`
	} `json:"Contacts,omitempty"`
	Links struct {
		X        string `json:"X,omitempty"`
		Linkedin string `json:"Linkedin,omitempty"`
	} `json:"Links,omitempty"`
	Education string `json:"Education,omitempty"`
}

func (c *UploadResume) Execute(ctx context.Context, path string) (uint8, error) {

	f, r, err := pdf.Open(path)
	if err != nil {
		panic(err)
	}
	defer f.Close()

	reader, err := r.GetPlainText()
	if err != nil {
		panic(err)
	}
	limitedReader := io.LimitReader(reader, 56*1024)

	buf := new(bytes.Buffer)
	_, err = io.Copy(buf, limitedReader)
	if err != nil {
		panic(err)
	}

	content := buf.String()
	normalized := strings.ReplaceAll(content, "\n", "")
	fmt.Println(normalized)
	getInfoFromText(normalized)

	return 0, nil
}

func getInfoFromText(text string) {
	apiKey := os.Getenv("GROQ_API")

	body := map[string]any{
		"model":       "openai/gpt-oss-20b",
		"temperature": 0.6,
		"messages": []map[string]string{
			{"role": "system", "content": "You are an intelligent assistant that extracts structured information from resumes. \nYour goal is to analyze raw resume text and convert it into a well-structured JSON object \nthat captures all the important details about the candidate.\n\nAlways ensure the output strictly follows the provided JSON schema, \neven if some fields are missing in the input (use null or empty arrays where appropriate).\nDo not include explanations, comments, or extra text — only return valid JSON."},
			{"role": "user", "content": fmt.Sprintf("Parse the following resume text and return a structured JSON object using this schema:\n\n{\n  \"Name\": string,\n  \"Surname\": string,\n  \"Summary\": string,\n  \"Experiences\": [\n    {\n      \"Company\": string,\n      \"TwoMostUsefulAchievements\": [string, string]\n    }\n  ],\n  \"Skills\": [string],\n  \"Contacts\": {\n    \"Mail\": string,\n    \"Phone\": string,\n    \"Address\": string\n  },\n  \"Links\": {\n    \"X\": string,\n    \"Linkedin\": string\n  },\n  \"Education\": string\n}\n\nResume text:\n%v\n\nBe concise in achievements and ensure names, emails, and URLs are extracted accurately. \nIf certain data cannot be found, leave it as null or an empty string.", text)},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	data, _ := json.Marshal(body)

	req, _ := http.NewRequest("POST", "https://api.groq.com/openai/v1/chat/completions", bytes.NewBuffer(data))
	req.Header.Set("Authorization", "Bearer "+apiKey)
	req.Header.Set("Content-Type", "application/json")

	resp, _ := http.DefaultClient.Do(req)
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusOK {
		fmt.Printf("HTTP %d: %s\n", resp.StatusCode, string(bodyBytes))
		return
	}

	// Define a minimal struct to extract only the content field
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		fmt.Println("JSON parse error:", err)
		fmt.Println("Raw response:", string(bodyBytes))
		return
	}

	if len(result.Choices) > 0 {
		fmt.Println(result.Choices[0].Message.Content)
	} else {
		fmt.Println("No choices found in response.")
	}
}
