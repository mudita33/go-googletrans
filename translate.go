package translator

import (
	"crypto/tls"
	"encoding/json"
	"fmt"
	"github.com/brianvoe/gofakeit/v6"
	"io"
	"math/rand"
	"net/http"
	"net/url"
	"strings"
)

// Config basic config.
type Config struct {
	ServiceUrls           []string
	UserAgent             []string
	Proxy                 []string
	UseUserAgentGenerator *bool
}

// Translated result object.
type Translated struct {
	Src    string // source language
	Dest   string // destination language
	Origin string // original text
	Text   string // translated text
}

type sentences struct {
	Sentences []sentence `json:"sentences"`
}

type sentence struct {
	Trans   string `json:"trans"`
	Orig    string `json:"orig"`
	Backend int    `json:"backend"`
}

type Provider struct {
	config Config
	faker  *gofakeit.Faker
}

type Translator struct {
	host   string
	client *http.Client
	ta     *tokenAcquirer
}

func randomChoose(slice []string) string {
	return slice[rand.Intn(len(slice))]
}

type addHeaderTransport struct {
	T              http.RoundTripper
	defaultHeaders map[string]string
}

func (adt *addHeaderTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range adt.defaultHeaders {
		req.Header.Add(k, v)
	}
	return adt.T.RoundTrip(req)
}

func newAddHeaderTransport(T http.RoundTripper, defaultHeaders map[string]string) *addHeaderTransport {
	if T == nil {
		T = http.DefaultTransport
	}
	return &addHeaderTransport{T, defaultHeaders}
}

func New(config ...Config) *Provider {

	var c Config
	if len(config) > 0 {
		c = config[0]
	}

	if c.UseUserAgentGenerator != nil && *c.UseUserAgentGenerator {
		return &Provider{
			faker:  gofakeit.NewCrypto(),
			config: c,
		}
	}

	return &Provider{
		faker:  nil,
		config: c,
	}

}

func (p *Provider) Client(config ...Config) *Translator {

	var c Config

	if len(p.config.Proxy) > 0 {
		c.Proxy = p.config.Proxy
	}

	c.UseUserAgentGenerator = p.config.UseUserAgentGenerator

	if len(p.config.UserAgent) > 0 {
		c.UserAgent = p.config.UserAgent
	}

	if len(p.config.ServiceUrls) > 0 {
		c.ServiceUrls = p.config.ServiceUrls
	}

	//--
	if len(config) > 0 {
		conf := config[0]
		if len(conf.Proxy) > 0 {
			c.Proxy = conf.Proxy
		}

		if conf.UseUserAgentGenerator != nil {
			c.UseUserAgentGenerator = conf.UseUserAgentGenerator
		}

		if len(conf.UserAgent) > 0 {
			c.UserAgent = conf.UserAgent
		}

		if len(conf.ServiceUrls) > 0 {
			c.ServiceUrls = conf.ServiceUrls
		}
	}

	// set default value
	if len(c.ServiceUrls) == 0 {
		c.ServiceUrls = []string{"translate.google.com"}
	}
	if len(c.UserAgent) == 0 {
		c.UserAgent = []string{defaultUserAgent}
	}

	var userAgent string

	if c.UseUserAgentGenerator != nil && *c.UseUserAgentGenerator && p.faker != nil {
		userAgent = p.faker.UserAgent()
	} else {
		userAgent = randomChoose(c.UserAgent)
	}

	host := randomChoose(c.ServiceUrls)

	var proxy string

	if len(c.Proxy) > 0 {
		proxy = randomChoose(c.Proxy)
	}

	transport := &http.Transport{}
	// Skip verifies the server's certificate chain and host name.
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true} // skip verify
	// Check & set proxy
	if strings.HasPrefix(proxy, "http") || strings.HasPrefix(proxy, "socks") {
		proxyUrl, _ := url.Parse(proxy)
		transport.Proxy = http.ProxyURL(proxyUrl) // set proxy
	}

	// new client with custom headers
	client := &http.Client{
		Transport: newAddHeaderTransport(transport, map[string]string{
			"User-Agent": userAgent,
		}),
	}

	ta := Token(host, client)
	return &Translator{
		host:   host,
		client: client,
		ta:     ta,
	}
}

// Translate given content.
// Set src to `auto` and system will attempt to identify the source language automatically.
func (a *Translator) Translate(origin, src, dest string) (*Translated, error) {
	// check src & dest
	src = strings.ToLower(src)
	dest = strings.ToLower(dest)
	//if _, ok := languages[src]; !ok {
	//	return nil, fmt.Errorf("src language code error")
	//}
	//if val, ok := languages[dest]; !ok || val == "auto" {
	//	return nil, fmt.Errorf("dest language code error")
	//}

	text, err := a.translate(a.client, origin, src, dest)
	if err != nil {
		return nil, err
	}
	result := &Translated{
		Src:    src,
		Dest:   dest,
		Origin: origin,
		Text:   text,
	}
	return result, nil
}

func (a *Translator) translate(client *http.Client, origin, src, dest string) (string, error) {
	tk, err := a.ta.do(origin)
	if err != nil {
		return "", err
	}

	tranUrl := fmt.Sprintf("https://%s/translate_a/single", a.host)
	req, err := http.NewRequest("GET", tranUrl, nil)
	if err != nil {
		return "", err
	}
	q := req.URL.Query()
	// params from chrome translate extension
	params := buildParams(origin, src, dest, tk)
	for i := range params {
		q.Add(i, params[i])
	}
	q.Add("dt", "t")
	q.Add("dt", "bd")
	q.Add("dj", "1")
	q.Add("source", "popup")
	req.URL.RawQuery = q.Encode()

	// do request
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == 200 {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		var sentences sentences
		err = json.Unmarshal(body, &sentences)
		if err != nil {
			return "", err
		}

		translated := ""
		// parse trans
		for _, s := range sentences.Sentences {
			translated += s.Trans
		}
		return translated, nil
	} else {
		return "", fmt.Errorf("expected statusCode 200, got: %d; resp: %+v", resp.StatusCode, resp)
	}
}

func buildParams(query, src, dest, token string) map[string]string {
	params := map[string]string{
		"client": "gtx",
		"sl":     src,
		"tl":     dest,
		"hl":     dest,
		"tk":     token,
		"q":      query,
	}
	return params
}
