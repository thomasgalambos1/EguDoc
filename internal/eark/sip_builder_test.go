package eark_test

import (
	"archive/zip"
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/eguilde/egudoc/internal/eark"
)

func TestBuildSIPContainsRequiredFiles(t *testing.T) {
	input := eark.SIPInput{
		PackageID:        "test-uuid-001",
		Label:            "INT/000001/2025 - Document test",
		DocumentContent:  []byte("%PDF-1.4 fake pdf"),
		DocumentFilename: "document.pdf",
		Metadata: eark.DocumentMetadata{
			NrInregistrare:      "INT/000001/2025",
			EmitentDenumire:     "Cetățean Test",
			InstitutionDenumire: "Primăria Costești",
			InstitutionCUI:      "RO12345678",
			AssignedUserName:    "Ion Ionescu",
			TipDocument:         "INTRARE",
			Clasificare:         "PUBLIC",
			CuvinteChecheie:     []string{"test", "document"},
			DataInregistrare:    time.Now(),
			TermenPastrareAni:   10,
			Obiect:              "Test document",
			Continut:            "Test content",
		},
		Events: []eark.WorkflowEventForPREMIS{
			{Action: "CREATE", ActorSubject: "user-001", Detail: "Document registered", OccurredAt: time.Now()},
		},
	}

	zipData, err := eark.BuildSIP(context.Background(), input)
	if err != nil {
		t.Fatalf("BuildSIP failed: %v", err)
	}

	r, err := zip.NewReader(bytes.NewReader(zipData), int64(len(zipData)))
	if err != nil {
		t.Fatalf("invalid zip: %v", err)
	}

	required := map[string]bool{
		"test-uuid-001/METS.xml":                                   false,
		"test-uuid-001/metadata/descriptive/dc.xml":               false,
		"test-uuid-001/metadata/preservation/premis.xml":          false,
		"test-uuid-001/representations/rep-001/data/document.pdf": false,
	}
	for _, f := range r.File {
		if _, ok := required[f.Name]; ok {
			required[f.Name] = true
		}
	}
	for path, found := range required {
		if !found {
			t.Errorf("required file missing from SIP: %s", path)
		}
	}
}
