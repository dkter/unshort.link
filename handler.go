package main

import (
	"fmt"
	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var schemeReplacer *strings.Replacer

func init() {
	schemeReplacer = strings.NewReplacer("https:/", "https://", "http:/", "http://")
}

type TemplateVars struct {
	ServerUrl string
	ShortUrl  string
	LongUrl   string
	Error     string
	LinkCount int
}

func handleIndex(rw http.ResponseWriter) {
	linkCount, err := getLinkCount()
	if err != nil {
		handleError(rw, errors.Wrap(err, "Could not get link count"))
		return
	}

	err = renderTemplate(rw,
		append(
			_escFSMustByte(useLocal, "/static/index.html"),
			_escFSMustByte(useLocal, "/static/main.html")...,
		),
		TemplateVars{ServerUrl: serveUrl, LinkCount: linkCount},
	)
	if err != nil {
		handleError(rw, errors.Wrap(err, "Could not render template"))
		return
	}
}

func handleShowRedirectPage(rw http.ResponseWriter, url *UnShortUrl) {
	err := renderTemplate(rw,
		append(
			_escFSMustByte(useLocal, "/static/show.html"),
			_escFSMustByte(useLocal, "/static/main.html")...,
		),
		TemplateVars{LongUrl: url.LongUrl.String(), ShortUrl: url.ShortUrl.String()},
	)
	if err != nil {
		handleError(rw, err)
		return
	}
}
func handleShowBlacklistPage(rw http.ResponseWriter, url *UnShortUrl) {
	err := renderTemplate(rw,
		append(
			_escFSMustByte(useLocal, "/static/blacklist.html"),
			_escFSMustByte(useLocal, "/static/main.html")...,
		),
		TemplateVars{LongUrl: url.LongUrl.String(), ShortUrl: url.ShortUrl.String()},
	)
	if err != nil {
		handleError(rw, err)
		return
	}
}

func handleError(rw http.ResponseWriter, err error) {
	fmt.Fprintf(rw, "An error occured: %s", err)
}

func renderTemplate(rw io.Writer, templateBytes []byte, vars TemplateVars) error {
	var err error
	mainTemplate := template.New("main")
	mainTemplate, err = mainTemplate.Parse(string(templateBytes))
	if err != nil {
		return errors.Wrap(err, "Could not parse tempalte")
	}

	err = mainTemplate.Execute(rw, vars)
	if err != nil {
		return errors.Wrap(err, "Could not execute tempalte")
	}
	return nil
}

func handleUnShort(rw http.ResponseWriter, req *http.Request, redirect bool) {
	baseUrl := strings.TrimPrefix(req.URL.String(), serveUrl)
	baseUrl = schemeReplacer.Replace(baseUrl)
	baseUrl = strings.TrimPrefix(baseUrl, "/")

	myUrl, err := url.Parse(baseUrl)
	if err != nil {
		handleError(rw, err)
		return
	}

	//Check in DB
	endUrl, err := getUrlFromDB(myUrl)
	if err != nil {
		logrus.Infof("Get new url from short link: '%s'", myUrl.String())

		endUrl, err = getUrl(myUrl)
		if err != nil {
			handleError(rw, err)
			return
		}

		// Check for blacklist
		if hostIsInBlacklist(endUrl.LongUrl.Host) {
			endUrl.Blacklisted = true
		}

		// Save to db
		err = saveUrlToDB(*endUrl)
	}
	logrus.Infof("Access url: '%v'", endUrl)

	if endUrl.Blacklisted {
		handleShowBlacklistPage(rw, endUrl)
		return
	}

	if !redirect {
		handleShowRedirectPage(rw, endUrl)
		return
	}

	http.Redirect(rw, req, endUrl.LongUrl.String(), http.StatusPermanentRedirect)
}
