package providers

import (
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"

	"github.com/bitly/oauth2_proxy/api"
)

type BitbucketProvider struct {
	*ProviderData
	Team string
	Repository string
}

func NewBitbucketProvider(p *ProviderData) *BitbucketProvider {
	p.ProviderName = "Bitbucket"
	if p.LoginURL == nil || p.LoginURL.String() == "" {
		p.LoginURL = &url.URL{
			Scheme: "https",
			Host:   "bitbucket.org",
			Path:   "/site/oauth2/authorize",
		}
	}
	if p.RedeemURL == nil || p.RedeemURL.String() == "" {
		p.RedeemURL = &url.URL{
			Scheme: "https",
			Host:   "bitbucket.org",
			Path:   "/site/oauth2/access_token",
		}
	}
	if p.ValidateURL == nil || p.ValidateURL.String() == "" {
		p.ValidateURL = &url.URL{
			Scheme: "https",
			Host:   "api.bitbucket.org",
			Path:   "/2.0/user/emails",
		}
	}
	if p.Scope == "" {
		p.Scope = "email"
	}
	return &BitbucketProvider{ProviderData: p}
}

func (p *BitbucketProvider) SetTeam(team string) {
	p.Team = team
	if ! strings.Contains(p.Scope, "team") {
		p.Scope += " team"
	}
}

func (p *BitbucketProvider) SetRepository(repository string) {
	p.Repository = repository
	if ! strings.Contains(p.Scope, "repository") {
		p.Scope += " repository"
	}
}

func debug(data []byte, err error) {
	if err == nil {
		fmt.Printf("%s\n\n", data)
	} else {
		log.Fatalf("%s\n\n", err)
	}
}

func (p *BitbucketProvider) GetEmailAddress(s *SessionState) (string, error) {

	var emails struct {
		Values []struct {
			Email   string `json:"email"`
			Primary bool   `json:"is_primary"`
		}
	}
	var teams struct {
		Values []struct {
			Name string `json:"username"`
		}
	}
	var repositories struct {
		Values []struct {
			FullName string `json:"full_name"`
		}
	}
	req, err := http.NewRequest("GET",
		p.ValidateURL.String()+"?access_token="+s.AccessToken, nil)
	if err != nil {
		log.Printf("failed building request %s", err)
		return "", err
	}
	err = api.RequestJson(req, &emails)
	if err != nil {
		log.Printf("failed making request %s", err)
		debug(httputil.DumpRequestOut(req, true))
		return "", err
	}

	if p.Team != "" {
		log.Printf("Filtering against membership in team %s\n", p.Team)
		teamURL := &url.URL{}
		*teamURL = *p.ValidateURL
		teamURL.Path = "/2.0/teams"
		req, err = http.NewRequest("GET",
			teamURL.String()+"?role=member&access_token="+s.AccessToken, nil)
		if err != nil {
			log.Printf("failed building request %s", err)
			return "", err
		}
		err = api.RequestJson(req, &teams)
		if err != nil {
			log.Printf("failed requesting teams membership %s", err)
			debug(httputil.DumpRequestOut(req, true))
			return "", err
		}
		var found = false
		log.Printf("%+v\n", teams)
		for _, team := range teams.Values {
			if p.Team == team.Name {
				found = true
				break
			}
		}
		if found != true {
			log.Printf("team membership test failed, access denied")
			return "", nil
		}
	}

	if p.Repository != "" {
		log.Printf("Filtering against access to repository %s\n", p.Repository)
		repositoriesURL := &url.URL{}
		*repositoriesURL = *p.ValidateURL
		repositoriesURL.Path = "/2.0/repositories/" + strings.Split(p.Repository, "/")[0]
		req, err = http.NewRequest("GET",
			repositoriesURL.String() + "?role=contributor" +
			"&q=full_name=" + url.QueryEscape("\"" + p.Repository + "\"") +
			"&access_token=" + s.AccessToken,
			nil)
		if err != nil {
			log.Printf("failed building request %s", err)
			return "", err
		}
		err = api.RequestJson(req, &repositories)
		if err != nil {
			log.Printf("failed checking repository access %s", err)
			debug(httputil.DumpRequestOut(req, true))
			return "", err
		}
		var found = false
		log.Printf("%+v\n", repositories)
		for _, repository := range repositories.Values {
			if p.Repository == repository.FullName {
				found = true
				break
			}
		}
		if found != true {
			log.Printf("repository access test failed, access denied")
			return "", nil
		}
	}

	for _, email := range emails.Values {
		if email.Primary {
			return email.Email, nil
		}
	}

	return "", nil
}
