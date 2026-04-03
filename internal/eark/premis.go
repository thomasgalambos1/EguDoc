package eark

import (
	"encoding/xml"
	"fmt"
	"time"
)

type PREMISRoot struct {
	XMLName xml.Name       `xml:"premis:premis"`
	XMLNS   string         `xml:"xmlns:premis,attr"`
	Version string         `xml:"version,attr"`
	Objects []PREMISObject `xml:"premis:object"`
	Events  []PREMISEvent  `xml:"premis:event"`
	Agents  []PREMISAgent  `xml:"premis:agent"`
}

type PREMISObject struct {
	XMLName          xml.Name `xml:"premis:object"`
	Type             string   `xml:"xsi:type,attr"`
	ObjectIdentifier struct {
		Type  string `xml:"premis:objectIdentifierType"`
		Value string `xml:"premis:objectIdentifierValue"`
	} `xml:"premis:objectIdentifier"`
	ObjectCharacteristics struct {
		CompositionLevel int `xml:"premis:compositionLevel"`
		Fixity           struct {
			MessageDigestAlgorithm string `xml:"premis:messageDigestAlgorithm"`
			MessageDigest          string `xml:"premis:messageDigest"`
		} `xml:"premis:fixity"`
		Size   int64 `xml:"premis:size"`
		Format struct {
			FormatDesignation struct {
				Name    string `xml:"premis:formatName"`
				Version string `xml:"premis:formatVersion"`
			} `xml:"premis:formatDesignation"`
			FormatRegistry struct {
				Name string `xml:"premis:formatRegistryName"`
				Key  string `xml:"premis:formatRegistryKey"`
				Role string `xml:"premis:formatRegistryRole"`
			} `xml:"premis:formatRegistry"`
		} `xml:"premis:format"`
	} `xml:"premis:objectCharacteristics"`
	OriginalName string `xml:"premis:originalName"`
}

type PREMISEvent struct {
	EventIdentifier struct {
		Type  string `xml:"premis:eventIdentifierType"`
		Value string `xml:"premis:eventIdentifierValue"`
	} `xml:"premis:eventIdentifier"`
	EventType     string `xml:"premis:eventType"`
	EventDateTime string `xml:"premis:eventDateTime"`
	EventDetail   string `xml:"premis:eventDetail"`
	EventOutcomeInformation struct {
		EventOutcome string `xml:"premis:eventOutcome"`
	} `xml:"premis:eventOutcomeInformation"`
	LinkingAgentIdentifier struct {
		Type  string `xml:"premis:linkingAgentIdentifierType"`
		Value string `xml:"premis:linkingAgentIdentifierValue"`
	} `xml:"premis:linkingAgentIdentifier"`
}

type PREMISAgent struct {
	AgentIdentifier struct {
		Type  string `xml:"premis:agentIdentifierType"`
		Value string `xml:"premis:agentIdentifierValue"`
	} `xml:"premis:agentIdentifier"`
	AgentName string `xml:"premis:agentName"`
	AgentType string `xml:"premis:agentType"`
}

type WorkflowEventForPREMIS struct {
	Action       string
	ActorSubject string
	Detail       string
	OccurredAt   time.Time
}

func BuildPREMIS(docID, filename, sha256 string, sizeBytes int64, ingestedAt time.Time, events []WorkflowEventForPREMIS, institutionCUI, creatingSystem string) ([]byte, error) {
	premis := PREMISRoot{
		XMLNS:   "http://www.loc.gov/premis/v3",
		Version: "3.0",
	}

	var obj PREMISObject
	obj.Type = "premis:file"
	obj.ObjectIdentifier.Type = "local"
	obj.ObjectIdentifier.Value = docID
	obj.ObjectCharacteristics.CompositionLevel = 0
	obj.ObjectCharacteristics.Fixity.MessageDigestAlgorithm = "SHA-256"
	obj.ObjectCharacteristics.Fixity.MessageDigest = sha256
	obj.ObjectCharacteristics.Size = sizeBytes
	obj.ObjectCharacteristics.Format.FormatDesignation.Name = "PDF/A-2b"
	obj.ObjectCharacteristics.Format.FormatDesignation.Version = "ISO 19005-2:2011"
	obj.ObjectCharacteristics.Format.FormatRegistry.Name = "PRONOM"
	obj.ObjectCharacteristics.Format.FormatRegistry.Key = "fmt/476"
	obj.ObjectCharacteristics.Format.FormatRegistry.Role = "specification"
	obj.OriginalName = filename
	premis.Objects = append(premis.Objects, obj)

	creationEvent := PREMISEvent{}
	creationEvent.EventIdentifier.Type = "local"
	creationEvent.EventIdentifier.Value = "creation-" + docID
	creationEvent.EventType = "creation"
	creationEvent.EventDateTime = ingestedAt.Format(time.RFC3339)
	creationEvent.EventDetail = "Document created in EguDoc registratura"
	creationEvent.EventOutcomeInformation.EventOutcome = "success"
	creationEvent.LinkingAgentIdentifier.Type = "software"
	creationEvent.LinkingAgentIdentifier.Value = creatingSystem
	premis.Events = append(premis.Events, creationEvent)

	for i, we := range events {
		event := PREMISEvent{}
		event.EventIdentifier.Type = "local"
		event.EventIdentifier.Value = fmt.Sprintf("workflow-%d-%s", i, docID)
		event.EventType = we.Action
		event.EventDateTime = we.OccurredAt.Format(time.RFC3339)
		event.EventDetail = we.Detail
		event.EventOutcomeInformation.EventOutcome = "success"
		event.LinkingAgentIdentifier.Type = "user"
		event.LinkingAgentIdentifier.Value = we.ActorSubject
		premis.Events = append(premis.Events, event)
	}

	submissionEvent := PREMISEvent{}
	submissionEvent.EventIdentifier.Type = "local"
	submissionEvent.EventIdentifier.Value = "submission-" + docID
	submissionEvent.EventType = "ingestion"
	submissionEvent.EventDateTime = time.Now().Format(time.RFC3339)
	submissionEvent.EventDetail = "Document submitted to EguDoc electronic archive"
	submissionEvent.EventOutcomeInformation.EventOutcome = "success"
	submissionEvent.LinkingAgentIdentifier.Type = "software"
	submissionEvent.LinkingAgentIdentifier.Value = "EguDoc/1.0"
	premis.Events = append(premis.Events, submissionEvent)

	premis.Agents = []PREMISAgent{
		{
			AgentIdentifier: struct {
				Type  string `xml:"premis:agentIdentifierType"`
				Value string `xml:"premis:agentIdentifierValue"`
			}{Type: "CUI", Value: institutionCUI},
			AgentName: "Instituție Publică",
			AgentType: "organization",
		},
		{
			AgentIdentifier: struct {
				Type  string `xml:"premis:agentIdentifierType"`
				Value string `xml:"premis:agentIdentifierValue"`
			}{Type: "software", Value: "EguDoc"},
			AgentName: "EguDoc Document Management System",
			AgentType: "software",
		},
	}

	out, err := xml.MarshalIndent(premis, "", "  ")
	if err != nil {
		return nil, err
	}
	return append([]byte(xml.Header), out...), nil
}
