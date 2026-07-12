package scan

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
)

type Confidence string

const (
	ConfidenceExact      Confidence = "EXACT"
	ConfidenceResolved   Confidence = "RESOLVED"
	ConfidenceNormalized Confidence = "NORMALIZED"
	ConfidenceInferred   Confidence = "INFERRED"
	ConfidenceWeak       Confidence = "WEAK"
	ConfidenceUnknown    Confidence = "UNKNOWN"
)

type Resolution string

const (
	ResolutionMatched    Resolution = "MATCHED"
	ResolutionPartial    Resolution = "PARTIAL"
	ResolutionUnresolved Resolution = "UNRESOLVED"
	ResolutionOutOfScope Resolution = "OUT_OF_SCOPE"
)

type Severity string

const (
	SeverityInfo    Severity = "INFO"
	SeverityWarning Severity = "WARNING"
	SeverityError   Severity = "ERROR"
)

type Coverage string

const (
	CoverageComplete    Coverage = "COMPLETE"
	CoveragePartial     Coverage = "PARTIAL"
	CoverageUnavailable Coverage = "UNAVAILABLE"
	CoverageFailed      Coverage = "FAILED"
)

type EvidenceLocation struct {
	Line   int `json:"line,omitempty"`
	Column int `json:"column,omitempty"`
}

type EvidenceRecord struct {
	ID         string           `json:"id"`
	Project    string           `json:"project"`
	File       string           `json:"file"`
	Start      EvidenceLocation `json:"start,omitempty"`
	End        EvidenceLocation `json:"end,omitempty"`
	Analyzer   string           `json:"analyzer"`
	Adapter    string           `json:"adapter,omitempty"`
	Method     string           `json:"method"`
	Reason     string           `json:"reason"`
	SourceHash string           `json:"source_hash,omitempty"`
}

func (value Confidence) Validate() error {
	switch value {
	case ConfidenceExact, ConfidenceResolved, ConfidenceNormalized, ConfidenceInferred, ConfidenceWeak, ConfidenceUnknown:
		return nil
	default:
		return fmt.Errorf("invalid confidence %q", value)
	}
}

func (value Resolution) Validate() error {
	switch value {
	case ResolutionMatched, ResolutionPartial, ResolutionUnresolved, ResolutionOutOfScope:
		return nil
	default:
		return fmt.Errorf("invalid resolution %q", value)
	}
}

func (value Severity) Validate() error {
	switch value {
	case SeverityInfo, SeverityWarning, SeverityError:
		return nil
	default:
		return fmt.Errorf("invalid severity %q", value)
	}
}

func (value Coverage) Validate() error {
	switch value {
	case CoverageComplete, CoveragePartial, CoverageUnavailable, CoverageFailed:
		return nil
	default:
		return fmt.Errorf("invalid coverage %q", value)
	}
}

func StableEvidenceID(record EvidenceRecord) string {
	parts := []string{
		record.Project,
		strings.TrimPrefix(strings.ReplaceAll(record.File, "\\", "/"), "/"),
		fmt.Sprintf("%d:%d-%d:%d", record.Start.Line, record.Start.Column, record.End.Line, record.End.Column),
		record.Analyzer,
		record.Adapter,
		record.Method,
		record.Reason,
		record.SourceHash,
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return "evidence:" + hex.EncodeToString(sum[:16])
}
