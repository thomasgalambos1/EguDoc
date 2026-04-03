package eark

import (
	"encoding/xml"
	"time"
)

type DublinCoreMetadata struct {
	XMLName   xml.Name `xml:"oai_dc:dc"`
	XMLNS     string   `xml:"xmlns:oai_dc,attr"`
	XMLNSDC   string   `xml:"xmlns:dc,attr"`
	XMLNSXSI  string   `xml:"xmlns:xsi,attr"`
	XSISchema string   `xml:"xsi:schemaLocation,attr"`

	Title       string   `xml:"dc:title"`
	Creator     string   `xml:"dc:creator"`
	Subject     []string `xml:"dc:subject"`
	Description string   `xml:"dc:description"`
	Publisher   string   `xml:"dc:publisher"`
	Contributor string   `xml:"dc:contributor"`
	Date        string   `xml:"dc:date"`
	Type        string   `xml:"dc:type"`
	Format      string   `xml:"dc:format"`
	Identifier  string   `xml:"dc:identifier"`
	Language    string   `xml:"dc:language"`
	Rights      string   `xml:"dc:rights"`
}

type DocumentMetadata struct {
	NrInregistrare      string
	EmitentDenumire     string
	InstitutionDenumire string
	InstitutionCUI      string
	AssignedUserName    string
	TipDocument         string
	Clasificare         string
	CuvinteChecheie     []string
	DataInregistrare    time.Time
	TermenPastrareAni   int
	Obiect              string
	Continut            string
}

func NewDublinCoreMetadata(meta DocumentMetadata) DublinCoreMetadata {
	return DublinCoreMetadata{
		XMLNS:       "http://www.openarchives.org/OAI/2.0/oai_dc/",
		XMLNSDC:     "http://purl.org/dc/elements/1.1/",
		XMLNSXSI:    "http://www.w3.org/2001/XMLSchema-instance",
		XSISchema:   "http://www.openarchives.org/OAI/2.0/oai_dc/ http://www.openarchives.org/OAI/2.0/oai_dc.xsd",
		Title:       meta.Obiect + " [" + meta.NrInregistrare + "]",
		Creator:     meta.EmitentDenumire,
		Subject:     meta.CuvinteChecheie,
		Description: meta.Continut,
		Publisher:   meta.InstitutionDenumire + " (CUI: " + meta.InstitutionCUI + ")",
		Contributor: meta.AssignedUserName,
		Date:        meta.DataInregistrare.Format("2006-01-02"),
		Type:        meta.TipDocument,
		Format:      "application/pdf",
		Identifier:  meta.NrInregistrare,
		Language:    "ro",
		Rights:      meta.Clasificare,
	}
}

func MarshalDublinCore(meta DocumentMetadata) ([]byte, error) {
	dc := NewDublinCoreMetadata(meta)
	out, err := xml.MarshalIndent(dc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
