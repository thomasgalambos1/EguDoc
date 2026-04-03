package eark

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"time"
)

type SIPPackage struct {
	PackageID string
	Files     map[string][]byte
}

type SIPInput struct {
	PackageID        string
	Label            string
	Metadata         DocumentMetadata
	Events           []WorkflowEventForPREMIS
	DocumentContent  []byte
	DocumentFilename string
	Attachments      []AttachmentInput
}

type AttachmentInput struct {
	Content     []byte
	Filename    string
	ContentType string
}

// BuildSIP generates a complete E-ARK CSIP SIP package as a ZIP archive.
func BuildSIP(ctx context.Context, input SIPInput) ([]byte, error) {
	createdAt := time.Now()

	dcXML, err := MarshalDublinCore(input.Metadata)
	if err != nil {
		return nil, fmt.Errorf("generate dublin core: %w", err)
	}
	dcHash := sha256hex(dcXML)

	docHash := sha256hex(input.DocumentContent)
	premisXML, err := BuildPREMIS(
		input.PackageID, input.DocumentFilename, docHash,
		int64(len(input.DocumentContent)), createdAt,
		input.Events, input.Metadata.InstitutionCUI, "EguDoc/1.0",
	)
	if err != nil {
		return nil, fmt.Errorf("generate premis: %w", err)
	}
	premisHash := sha256hex(premisXML)

	fileRefs := []FileRef{{
		Filename: input.DocumentFilename, ContentType: "application/pdf",
		Size: int64(len(input.DocumentContent)), SHA256: docHash,
	}}
	for _, att := range input.Attachments {
		fileRefs = append(fileRefs, FileRef{
			Filename: att.Filename, ContentType: att.ContentType,
			Size: int64(len(att.Content)), SHA256: sha256hex(att.Content),
		})
	}

	metsXML, err := BuildRootMETS(
		input.PackageID, input.Label,
		input.Metadata.InstitutionDenumire, input.Metadata.InstitutionCUI,
		int64(len(dcXML)), dcHash, int64(len(premisXML)), premisHash,
		fileRefs, createdAt,
	)
	if err != nil {
		return nil, fmt.Errorf("generate mets: %w", err)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)

	writeZipFile := func(path string, content []byte) error {
		w, err := zw.Create(path)
		if err != nil {
			return fmt.Errorf("create zip entry %q: %w", path, err)
		}
		_, err = w.Write(content)
		return err
	}

	packageDir := input.PackageID + "/"
	if err := writeZipFile(packageDir+"METS.xml", metsXML); err != nil {
		return nil, err
	}
	if err := writeZipFile(packageDir+"metadata/descriptive/dc.xml", dcXML); err != nil {
		return nil, err
	}
	if err := writeZipFile(packageDir+"metadata/preservation/premis.xml", premisXML); err != nil {
		return nil, err
	}
	if err := writeZipFile(packageDir+"representations/rep-001/data/"+input.DocumentFilename, input.DocumentContent); err != nil {
		return nil, err
	}
	for _, att := range input.Attachments {
		if err := writeZipFile(packageDir+"representations/rep-001/data/"+att.Filename, att.Content); err != nil {
			return nil, err
		}
	}

	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("close zip: %w", err)
	}
	return buf.Bytes(), nil
}

func BuildSIPAndStream(ctx context.Context, input SIPInput) (io.Reader, int64, error) {
	data, err := BuildSIP(ctx, input)
	if err != nil {
		return nil, 0, err
	}
	return bytes.NewReader(data), int64(len(data)), nil
}

func sha256hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
