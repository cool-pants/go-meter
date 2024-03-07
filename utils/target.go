package gogeta

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"io"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
)

func handleErrors(err error, msg string) {
	if err != nil {
		fmt.Println(msg)
		os.Exit(1)
	}
}

type TestConfig struct {
	Workers int `json:"workers" yaml:"workers"`
}

type RequestConfig struct {
	Method  string                 `yaml:"method"`
	Url     string                 `yaml:"url"`
	Headers map[string]string      `yaml:"headers"`
	Body    map[string]interface{} `yaml:"body"`
	Params  []string               `yaml:"params"`
}

type TargetSetup struct {
	PreRun  interface{}   `yaml:"preRun"`
	Run     RequestConfig `yaml:"run"`
	PostRun interface{}   `yaml:"postRun"`
}

type Plan struct {
	Name    string        `yaml:"name"`
	Targets []TargetSetup `yaml:"targets"`
}

type Config struct {
	ApiVersion    string     `json:"apiVersion" yaml:"apiVersion"`
	Configuration TestConfig `json:"config" yaml:"config"`
	TargetPlan    []Plan     `json:"targetPlan" yaml:"targetPlan"`
}

type Target struct {
	Method string      `json:"method"`
	URL    string      `json:"url"`
	Body   []byte      `json:"body,omitempty"`
	Header http.Header `json:"header,omitempty"`
	Next   *Target     `json:"next,omitempty"`
}

var (
	// ErrNoTargets is returned when not enough Targets are available.
	ErrNoTargets = errors.New("no targets to attack")
	// ErrNilTarget is returned when the passed Target pointer is nil.
	ErrNilTarget = errors.New("nil target")
	// ErrNoMethod is returned by JSONTargeter when a parsed Target has
	// no method.
	ErrNoMethod = errors.New("target: required method is missing")
	// ErrNoURL is returned by JSONTargeter when a parsed Target has no
	// URL.
	ErrNoURL = errors.New("target: required url is missing")
)

// A Targeter decodes a Target or returns an error in case of failure.
// Implementations must be safe for concurrent use.
type Targeter func(*Target) error

func NewStaticTargeter(tgts ...Target) Targeter {
	i := int64(-1)
	return func(tgt *Target) error {
		if tgt == nil {
			return ErrNilTarget
		}
		if tgt.Equal(&Target{}) {
			*tgt = tgts[atomic.AddInt64(&i, 1)%int64(len(tgts))]

		}
		return nil
	}
}

func ProcessReader(reader io.Reader) []Target {
	res, err := io.ReadAll(reader)
	handleErrors(err, fmt.Sprintf("Reading from STDIN failed with %v", err))

	var config Config

	err = yaml.Unmarshal(res, &config)
	handleErrors(err, fmt.Sprintf("Failure in unmarshalling plan %v", err))

	var tgts []Target

	for _, plan := range config.TargetPlan {
		for targetIndex := range plan.Targets {
			marshalledBody, err := json.Marshal(plan.Targets[targetIndex].Run.Body)
			handleErrors(err, fmt.Sprintf("Failure in marshalling Body %v", err))

			tgt := Target{
				Method: plan.Targets[targetIndex].Run.Method,
				URL:    plan.Targets[targetIndex].Run.Url,
				Body:   marshalledBody,
				Next:   nil,
			}
			tgt.Header = http.Header{}
			for k, v := range plan.Targets[targetIndex].Run.Headers {
				tgt.Header.Add(k, v)
			}

			if targetIndex != 0 {
				tgts[(len(tgts) - 1)].Next = &tgt
			} else {
				tgts = append(tgts, tgt)
			}
		}
	}
	return tgts
}

func substituteURL(url string, values ...any) (string, error) {
	if !strings.Contains(url, "%s") {
		// No placeholders, return the original URL
		return url, nil
	}

	if len(values) == 0 {
		return "", fmt.Errorf("url contains placeholders (%s) but no values provided")
	}

	// Escape the URL to avoid unintended substitution of other characters
	escapedURL := strings.ReplaceAll(url, "%", "%%")

	// Format the URL with the provided values
	formattedURL := fmt.Sprintf(escapedURL, values...)

	// Unescape the formatted URL
	return strings.ReplaceAll(formattedURL, "%%", "%"), nil
}

// Request creates an *http.Request out of Target and returns it along with an
// error in case of failure.
func (t *Target) Request(cache map[string]string) (*http.Request, error) {
	var body io.Reader
	if len(t.Body) != 0 {
		body = bytes.NewReader(t.Body)
	}

	val, ok := cache["txn_uuid"]
	if ok && strings.Contains(t.URL, "%s") {
		t.URL = fmt.Sprintf(t.URL, val)
	}

	req, err := http.NewRequest(t.Method, t.URL, body)
	if err != nil {
		return nil, err
	}

	for k, vs := range t.Header {
		req.Header[k] = make([]string, len(vs))
		copy(req.Header[k], vs)
	}

	if host := req.Header.Get("Host"); host != "" {
		req.Host = host
	}

	return req, nil
}

// Equal returns true if the target is equal to the other given target.
func (t *Target) Equal(other *Target) bool {
	switch {
	case t == other:
		return true
	case t == nil || other == nil:
		return false
	default:
		equal := t.Method == other.Method &&
			t.URL == other.URL &&
			bytes.Equal(t.Body, other.Body) &&
			len(t.Header) == len(other.Header)

		if !equal {
			return false
		}

		for k := range t.Header {
			left, right := t.Header[k], other.Header[k]
			if len(left) != len(right) {
				return false
			}
			for i := range left {
				if left[i] != right[i] {
					return false
				}
			}
		}

		return true
	}
}
