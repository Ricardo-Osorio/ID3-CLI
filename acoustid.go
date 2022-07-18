package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
)

const (
	// currently using one from the public docs as mine reports as being invalid
	API_KEY          = "_eaTSIacNA0"
	ACOUSTID_API_URL = "https://api.acoustid.org/v2/lookup"
)

type AcoustIDResponse struct {
	Results []Result
	Status  string
	Error   ReqError
}
type ReqError struct {
	Message string
}
type Result struct {
	Score      float64
	Recordings []struct {
		Artists []struct {
			Name       string
			JoinPhrase string
		}
		ReleaseGroups []struct {
			Type           string
			Title          string
			SecondaryTypes []string
		}
		Sources int
		Title   string
	}
}

func Request(duration int, fingerprint string) (*AcoustIDResponse, error) {
	query := fmt.Sprintf(
		"client=%s&duration=%d&meta=%s&fingerprint=%s",
		API_KEY,
		duration,
		"recordings+releasegroups+sources+compress",
		fingerprint,
	)

	values, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}

	client := http.Client{}
	response, err := client.PostForm(ACOUSTID_API_URL, values)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	body, err := ioutil.ReadAll(response.Body)
	if err != nil {
		return nil, err
	}

	// fmt.Printf("all body: %s\n", string(body))

	aidResp := &AcoustIDResponse{}
	err = json.Unmarshal(body, aidResp)
	if err != nil {
		return nil, err
	}

	if aidResp.Status != "ok" {
		return nil, fmt.Errorf(aidResp.Error.Message)
	}

	return aidResp, nil
}
