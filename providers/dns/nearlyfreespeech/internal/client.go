package internal

import (
	"crypto/sha1"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"
	"time"

	querystring "github.com/google/go-querystring/query"
)

const apiURL = "https://api.nearlyfreespeech.net"

const authenticationHeader = "X-NFSN-Authentication"

const saltBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"

type Record struct {
	Name string `url:"name,omitempty"`
	Type string `url:"type,omitempty"`
	Data string `url:"data,omitempty"`
	TTL  int    `url:"ttl,omitempty"`
}

type Client struct {
	HTTPClient *http.Client

	login   string
	apiKey  string
	baseURL *url.URL
}

func NewClient(login string, apiKey string) *Client {
	baseURL, _ := url.Parse(apiURL)
	return &Client{
		HTTPClient: &http.Client{Timeout: 10 * time.Second},
		login:      login,
		apiKey:     apiKey,
		baseURL:    baseURL,
	}
}

func (c Client) AddRecord(domain string, record Record) error {
	params, err := querystring.Values(record)
	if err != nil {
		return err
	}

	return c.do(path.Join("dns", domain, "addRR"), params)
}

func (c Client) RemoveRecord(domain string, record Record) error {
	params, err := querystring.Values(record)
	if err != nil {
		return err
	}

	return c.do(path.Join("dns", domain, "removeRR"), params)
}

func (c Client) do(uri string, params url.Values) error {
	endpoint, err := c.baseURL.Parse(path.Join(c.baseURL.Path, uri))
	if err != nil {
		return err
	}

	payload := params.Encode()

	req, err := http.NewRequest(http.MethodPost, endpoint.String(), strings.NewReader(payload))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set(authenticationHeader, c.createSignature(endpoint.Path, payload))

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return err
	}

	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("%s: %s", resp.Status, data)
	}

	return nil
}

func (c Client) createSignature(uri string, body string) string {
	// This is the only part of this that needs to be serialized.
	salt := make([]byte, 16)
	for i := 0; i < 16; i++ {
		salt[i] = saltBytes[rand.Intn(len(saltBytes))]
	}

	// Header is "login;timestamp;salt;hash".
	// hash is SHA1("login;timestamp;salt;api-key;request-uri;body-hash")
	// and body-hash is SHA1(body).

	bodyHash := sha1.Sum([]byte(body))
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	hashInput := fmt.Sprintf("%s;%s;%s;%s;%s;%02x", c.login, timestamp, salt, c.apiKey, uri, bodyHash)

	return fmt.Sprintf("%s;%s;%s;%02x", c.login, timestamp, salt, sha1.Sum([]byte(hashInput)))
}
