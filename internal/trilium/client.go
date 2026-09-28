package trilium

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	stdhtml "html"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	xhtml "golang.org/x/net/html"

	"showmethestory/internal/fsutil"
)

const maxResponse = 2 << 20

var noteIDPattern = regexp.MustCompile(`^[A-Za-z0-9_]{4,32}$`)

type Config struct {
	BaseURL string `json:"base_url"`
	Token   string `json:"token"`
}

type Note struct {
	NoteID      string `json:"noteId"`
	Title       string `json:"title"`
	Type        string `json:"type"`
	IsProtected bool   `json:"isProtected"`
}

type Client struct {
	baseURL string
	token   string
	http    *http.Client
}

func ConfigPath(dataDir string) string { return filepath.Join(dataDir, "trilium.json") }

func LoadConfig(path string) (Config, error) {
	var cfg Config
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return cfg, nil
	}
	if err != nil {
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	if cfg.BaseURL != "" {
		cfg.BaseURL, err = NormalizeBaseURL(cfg.BaseURL)
	}
	return cfg, err
}

func SaveConfig(path string, cfg Config) error {
	var err error
	if cfg.BaseURL != "" {
		cfg.BaseURL, err = NormalizeBaseURL(cfg.BaseURL)
		if err != nil {
			return err
		}
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := fsutil.WriteFileAtomic(path, data); err != nil {
		return err
	}
	return os.Chmod(path, 0600)
}

func NormalizeBaseURL(raw string) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return "", errors.New("invalid Trilium server URL")
	}
	path := strings.TrimRight(u.EscapedPath(), "/")
	if strings.HasSuffix(path, "/etapi") {
		path = strings.TrimSuffix(path, "/etapi")
	}
	u.Path = path
	u.RawPath = ""
	return strings.TrimRight(u.String(), "/"), nil
}

func ValidNoteID(id string) bool { return noteIDPattern.MatchString(id) }

func NewClient(cfg Config) (*Client, error) {
	base, err := NormalizeBaseURL(cfg.BaseURL)
	if err != nil || cfg.Token == "" {
		return nil, errors.New("Trilium connection is not configured")
	}
	return &Client{baseURL: base, token: cfg.Token, http: &http.Client{
		Timeout:       15 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("Trilium redirect refused") },
	}}, nil
}

func (c *Client) BaseURL() string { return c.baseURL }

func (c *Client) request(method, path string, body []byte) ([]byte, error) {
	req, err := http.NewRequest(method, c.baseURL+"/etapi"+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", c.token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("Trilium returned HTTP %d", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponse+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxResponse {
		return nil, errors.New("Trilium response exceeds 2 MiB")
	}
	return data, nil
}

func (c *Client) Test() error {
	_, err := c.request(http.MethodGet, "/app-info", nil)
	return err
}

func (c *Client) GetNote(id string) (Note, string, error) {
	if !ValidNoteID(id) {
		return Note{}, "", errors.New("invalid Trilium note ID")
	}
	data, err := c.request(http.MethodGet, "/notes/"+id, nil)
	if err != nil {
		return Note{}, "", err
	}
	var note Note
	if err := json.Unmarshal(data, &note); err != nil {
		return Note{}, "", err
	}
	if note.NoteID != id {
		return Note{}, "", errors.New("Trilium returned a different note ID")
	}
	if note.Type != "text" || note.IsProtected {
		return Note{}, "", errors.New("Trilium outline must be an accessible text note")
	}
	data, err = c.request(http.MethodGet, "/notes/"+id+"/content", nil)
	if err != nil {
		return Note{}, "", err
	}
	content := PlainText(string(data))
	if strings.TrimSpace(content) == "" {
		return Note{}, "", errors.New("Trilium outline note is empty")
	}
	return note, content, nil
}

func (c *Client) CreateTextNote(parentID, title, content string) (string, error) {
	if !ValidNoteID(parentID) {
		return "", errors.New("invalid Trilium parent note ID")
	}
	data, err := json.Marshal(map[string]string{"parentNoteId": parentID, "title": title, "type": "text", "content": content})
	if err != nil {
		return "", err
	}
	data, err = c.request(http.MethodPost, "/create-note", data)
	if err != nil {
		return "", err
	}
	var result struct {
		Note Note `json:"note"`
	}
	if err := json.Unmarshal(data, &result); err != nil {
		return "", err
	}
	if !ValidNoteID(result.Note.NoteID) {
		return "", errors.New("Trilium returned no note ID")
	}
	return result.Note.NoteID, nil
}

// PlainText retains useful block boundaries from Trilium text-note HTML.
func PlainText(content string) string {
	node, err := xhtml.Parse(strings.NewReader(content))
	if err != nil {
		return strings.TrimSpace(content)
	}
	var out strings.Builder
	var walk func(*xhtml.Node)
	walk = func(n *xhtml.Node) {
		if n.Type == xhtml.ElementNode && (n.Data == "script" || n.Data == "style") {
			return
		}
		block := n.Type == xhtml.ElementNode && (n.Data == "p" || n.Data == "div" || n.Data == "li" || n.Data == "h1" || n.Data == "h2" || n.Data == "h3" || n.Data == "br")
		if block && out.Len() > 0 {
			out.WriteByte('\n')
		}
		if n.Type == xhtml.TextNode {
			out.WriteString(n.Data)
		}
		for child := n.FirstChild; child != nil; child = child.NextSibling {
			walk(child)
		}
	}
	walk(node)
	lines := strings.Split(out.String(), "\n")
	for i := range lines {
		lines[i] = strings.TrimSpace(lines[i])
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

type ArticleChapter struct {
	Num            int
	Title, Content string
}

func ArticleHTML(chapters []ArticleChapter, english bool) string {
	var out strings.Builder
	for _, chapter := range chapters {
		if english {
			fmt.Fprintf(&out, "<h2>Chapter %d: %s</h2>", chapter.Num, stdhtml.EscapeString(chapter.Title))
		} else {
			fmt.Fprintf(&out, "<h2>第 %d 章 %s</h2>", chapter.Num, stdhtml.EscapeString(chapter.Title))
		}
		for _, paragraph := range strings.Split(strings.ReplaceAll(chapter.Content, "\r\n", "\n"), "\n\n") {
			if strings.TrimSpace(paragraph) == "" {
				continue
			}
			out.WriteString("<p>")
			out.WriteString(strings.ReplaceAll(stdhtml.EscapeString(strings.TrimSpace(paragraph)), "\n", "<br>"))
			out.WriteString("</p>")
		}
	}
	return out.String()
}
