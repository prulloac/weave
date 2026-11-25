package audit

import (
	"encoding/json"
	"sort"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

const sarifSchemaURI = "https://json.schemastore.org/sarif-2.1.0.json"

type sarifLog struct {
	Schema  string     `json:"$schema"`
	Version string     `json:"version"`
	Runs    []sarifRun `json:"runs"`
}

type sarifRun struct {
	Tool    sarifTool     `json:"tool"`
	Results []sarifResult `json:"results"`
}

type sarifTool struct {
	Driver sarifDriver `json:"driver"`
}

type sarifDriver struct {
	Name           string      `json:"name"`
	Version        string      `json:"version,omitempty"`
	InformationURI string      `json:"informationUri,omitempty"`
	Rules          []sarifRule `json:"rules,omitempty"`
}

type sarifRule struct {
	ID               string             `json:"id"`
	ShortDescription sarifMessageObject `json:"shortDescription"`
}

type sarifResult struct {
	RuleID    string          `json:"ruleId"`
	Level     string          `json:"level"`
	Message   sarifMessage    `json:"message"`
	Locations []sarifLocation `json:"locations,omitempty"`
}

type sarifMessage struct {
	Text string `json:"text"`
}

type sarifMessageObject struct {
	Text string `json:"text"`
}

type sarifLocation struct {
	PhysicalLocation sarifPhysicalLocation `json:"physicalLocation"`
}

type sarifPhysicalLocation struct {
	ArtifactLocation sarifArtifactLocation `json:"artifactLocation"`
}

type sarifArtifactLocation struct {
	URI string `json:"uri"`
}

// RenderSARIF converts an audit report into a deterministic SARIF 2.1.0 log.
func RenderSARIF(report contractv1.AuditReport, version string) ([]byte, error) {
	rules := make(map[string]bool, len(report.Findings))
	results := make([]sarifResult, 0, len(report.Findings))
	for _, finding := range report.Findings {
		rules[string(finding.Code)] = true
		result := sarifResult{
			RuleID:  string(finding.Code),
			Level:   sarifLevel(finding.Severity),
			Message: sarifMessage{Text: finding.Message},
		}
		if path := sarifPath(finding); path != "" {
			result.Locations = []sarifLocation{{
				PhysicalLocation: sarifPhysicalLocation{
					ArtifactLocation: sarifArtifactLocation{URI: path},
				},
			}}
		}
		results = append(results, result)
	}
	sort.SliceStable(results, func(i, j int) bool {
		left, right := sarifURI(results[i]), sarifURI(results[j])
		if left != right {
			return left < right
		}
		if results[i].RuleID != results[j].RuleID {
			return results[i].RuleID < results[j].RuleID
		}
		return results[i].Message.Text < results[j].Message.Text
	})
	ruleList := make([]sarifRule, 0, len(rules))
	for code := range rules {
		ruleList = append(ruleList, sarifRule{ID: code, ShortDescription: sarifMessageObject{Text: code}})
	}
	sort.Slice(ruleList, func(i, j int) bool { return ruleList[i].ID < ruleList[j].ID })
	log := sarifLog{
		Schema:  sarifSchemaURI,
		Version: "2.1.0",
		Runs: []sarifRun{{
			Tool: sarifTool{Driver: sarifDriver{
				Name:           "weave",
				Version:        version,
				InformationURI: "https://github.com/prulloac/weave",
				Rules:          ruleList,
			}},
			Results: results,
		}},
	}
	data, err := json.MarshalIndent(log, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func sarifLevel(severity contractv1.AuditSeverity) string {
	switch severity {
	case contractv1.AuditSeverityError:
		return "error"
	case contractv1.AuditSeverityWarning:
		return "warning"
	default:
		return "note"
	}
}

func sarifPath(finding contractv1.AuditFinding) string {
	path := strings.TrimSpace(finding.Source)
	if path == "" {
		path = strings.TrimSpace(finding.Concept)
	}
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.TrimPrefix(path, "/")
	return path
}

func sarifURI(result sarifResult) string {
	if len(result.Locations) == 0 {
		return ""
	}
	return result.Locations[0].PhysicalLocation.ArtifactLocation.URI
}
