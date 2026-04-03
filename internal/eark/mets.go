package eark

import (
	"encoding/xml"
	"fmt"
	"time"
)

type METS struct {
	XMLName    xml.Name      `xml:"mets"`
	XMLNS      string        `xml:"xmlns,attr"`
	XMLNSXlink string        `xml:"xmlns:xlink,attr"`
	XMLNSCSIP  string        `xml:"xmlns:csip,attr"`
	XMLNSSIP   string        `xml:"xmlns:sip,attr"`
	OBJID      string        `xml:"OBJID,attr"`
	LABEL      string        `xml:"LABEL,attr"`
	PROFILE    string        `xml:"PROFILE,attr"`
	TYPE       string        `xml:"TYPE,attr"`
	CSIPType   string        `xml:"csip:CONTENTINFORMATIONTYPE,attr"`
	MetsHdr    METSHeader    `xml:"metsHdr"`
	DmdSec     []METSDmd     `xml:"dmdSec"`
	AmdSec     METSAmd       `xml:"amdSec"`
	FileSec    METSFileSec   `xml:"fileSec"`
	StructMap  METSStructMap `xml:"structMap"`
}

type METSHeader struct {
	CREATEDATE          string      `xml:"CREATEDATE,attr"`
	RECORDSTATUS        string      `xml:"RECORDSTATUS,attr"`
	CSIPOAISPackageType string      `xml:"csip:OAISPACKAGETYPE,attr"`
	Agents              []METSAgent `xml:"agent"`
}

type METSAgent struct {
	ROLE      string     `xml:"ROLE,attr"`
	TYPE      string     `xml:"TYPE,attr"`
	OTHERTYPE string     `xml:"OTHERTYPE,attr,omitempty"`
	Name      string     `xml:"name"`
	Note      []METSNote `xml:"note"`
}

type METSNote struct {
	NoteType string `xml:"csip:NOTETYPE,attr"`
	Value    string `xml:",chardata"`
}

type METSDmd struct {
	ID      string    `xml:"ID,attr"`
	CREATED string    `xml:"CREATED,attr"`
	MdRef   METSMdRef `xml:"mdRef"`
}

type METSAmd struct {
	DigiProvMD []METSDigiProv `xml:"digiprovMD"`
}

type METSDigiProv struct {
	ID    string    `xml:"ID,attr"`
	MdRef METSMdRef `xml:"mdRef"`
}

type METSMdRef struct {
	LOCTYPE      string `xml:"LOCTYPE,attr"`
	XlinkHref    string `xml:"xlink:href,attr"`
	MDTYPE       string `xml:"MDTYPE,attr"`
	MIMETYPE     string `xml:"MIMETYPE,attr"`
	SIZE         int64  `xml:"SIZE,attr"`
	CHECKSUM     string `xml:"CHECKSUM,attr"`
	CHECKSUMTYPE string `xml:"CHECKSUMTYPE,attr"`
}

type METSFileSec struct {
	FileGrps []METSFileGrp `xml:"fileGrp"`
}

type METSFileGrp struct {
	USE   string     `xml:"USE,attr"`
	Files []METSFile `xml:"file"`
}

type METSFile struct {
	ID           string     `xml:"ID,attr"`
	MIMETYPE     string     `xml:"MIMETYPE,attr"`
	SIZE         int64      `xml:"SIZE,attr"`
	CREATED      string     `xml:"CREATED,attr"`
	CHECKSUM     string     `xml:"CHECKSUM,attr"`
	CHECKSUMTYPE string     `xml:"CHECKSUMTYPE,attr"`
	FLocat       METSFLocat `xml:"FLocat"`
}

type METSFLocat struct {
	LOCTYPE   string `xml:"LOCTYPE,attr"`
	XlinkHref string `xml:"xlink:href,attr"`
}

type METSStructMap struct {
	TYPE  string  `xml:"TYPE,attr"`
	LABEL string  `xml:"LABEL,attr"`
	Div   METSDiv `xml:"div"`
}

type METSDiv struct {
	LABEL string    `xml:"LABEL,attr"`
	Divs  []METSDiv `xml:"div,omitempty"`
}

type FileRef struct {
	Filename    string
	ContentType string
	Size        int64
	SHA256      string
}

func BuildRootMETS(packageID, label, institutionName, institutionCUI string, dcXMLSize int64, dcXMLHash string, premisXMLSize int64, premisXMLHash string, docFiles []FileRef, createdAt time.Time) ([]byte, error) {
	mets := METS{
		XMLNS:      "http://www.loc.gov/METS/",
		XMLNSXlink: "http://www.w3.org/1999/xlink",
		XMLNSCSIP:  "https://earkcsip.dilcis.eu/schema/",
		XMLNSSIP:   "https://earksip.dilcis.eu/schema/",
		OBJID:      packageID,
		LABEL:      label,
		PROFILE:    "https://earkcsip.dilcis.eu/profile/E-ARK-CSIP.xml",
		TYPE:       "OTHER",
		CSIPType:   "MIXED",
	}

	mets.MetsHdr = METSHeader{
		CREATEDATE:          createdAt.Format(time.RFC3339),
		RECORDSTATUS:        "NEW",
		CSIPOAISPackageType: "SIP",
		Agents: []METSAgent{
			{ROLE: "CREATOR", TYPE: "ORGANIZATION", Name: institutionName, Note: []METSNote{{NoteType: "IDENTIFICATIONCODE", Value: institutionCUI}}},
			{ROLE: "CREATOR", TYPE: "OTHER", OTHERTYPE: "SOFTWARE", Name: "EguDoc", Note: []METSNote{{NoteType: "SOFTWARE VERSION", Value: "1.0"}}},
		},
	}

	mets.DmdSec = []METSDmd{{
		ID:      "dmd-dc-001",
		CREATED: createdAt.Format(time.RFC3339),
		MdRef:   METSMdRef{LOCTYPE: "URL", XlinkHref: "metadata/descriptive/dc.xml", MDTYPE: "DC", MIMETYPE: "text/xml", SIZE: dcXMLSize, CHECKSUM: dcXMLHash, CHECKSUMTYPE: "SHA-256"},
	}}

	mets.AmdSec = METSAmd{DigiProvMD: []METSDigiProv{{
		ID:    "digiprov-premis-001",
		MdRef: METSMdRef{LOCTYPE: "URL", XlinkHref: "metadata/preservation/premis.xml", MDTYPE: "PREMIS", MIMETYPE: "text/xml", SIZE: premisXMLSize, CHECKSUM: premisXMLHash, CHECKSUMTYPE: "SHA-256"},
	}}}

	var files []METSFile
	for i, f := range docFiles {
		files = append(files, METSFile{
			ID: fmt.Sprintf("file-%03d", i+1), MIMETYPE: f.ContentType, SIZE: f.Size,
			CREATED: createdAt.Format(time.RFC3339), CHECKSUM: f.SHA256, CHECKSUMTYPE: "SHA-256",
			FLocat: METSFLocat{LOCTYPE: "URL", XlinkHref: "representations/rep-001/data/" + f.Filename},
		})
	}
	mets.FileSec = METSFileSec{FileGrps: []METSFileGrp{{USE: "Representations/rep-001", Files: files}}}
	mets.StructMap = METSStructMap{
		TYPE: "PHYSICAL", LABEL: "CSIP",
		Div: METSDiv{LABEL: "Root", Divs: []METSDiv{{LABEL: "Metadata"}, {LABEL: "Representations", Divs: []METSDiv{{LABEL: "rep-001"}}}}},
	}

	out, err := xml.MarshalIndent(mets, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
