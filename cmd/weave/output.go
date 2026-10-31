package main

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine/audit"
	"github.com/prulloac/weave/engine/operations"
)

func usageError(stdout io.Writer, stderr io.Writer, jsonMode bool, message string) int {
	if jsonMode {
		envelope := contractv1.FailureEnvelope{
			Contract: operations.ContractVersion,
			CLI:      operations.CLIVersion,
			OK:       false,
			Error: contractv1.OperationError{
				Code:    contractv1.ErrInvalidRequest,
				Message: message,
			},
		}
		_ = writeJSON(stdout, envelope)
	}
	printOperationError(stderr, contractv1.ErrInvalidRequest, message)
	return 2
}

func writeJSON(writer io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(writer, "%s\n", data)
	return err
}

func successExitCode(op contractv1.OperationName, result any) int {
	switch op {
	case contractv1.OpAudit:
		report, ok := result.(contractv1.AuditReport)
		if ok && audit.Failed(report) {
			return 1
		}
	case contractv1.OpIngest, contractv1.OpRebuild:
		report, ok := result.(contractv1.IngestReport)
		if ok && report.Errors > 0 {
			return 1
		}
	case contractv1.OpSync:
		report, ok := result.(contractv1.SyncReport)
		if ok && report.Errors > 0 {
			return 1
		}
	}
	return 0
}

func isEmptyResult(op contractv1.OperationName, result any) bool {
	switch op {
	case contractv1.OpSearch:
		response, ok := result.(contractv1.SearchResponse)
		if !ok {
			return false
		}
		if response.Mode == contractv1.SearchBM25 {
			return len(response.Lexical) == 0
		}
		switch response.Mode {
		case contractv1.SearchKeyword:
			return len(response.Keyword) == 0
		case contractv1.SearchFuzzy:
			return len(response.Fuzzy) == 0
		default:
			return len(response.Hybrid) == 0
		}
	default:
		return false
	}
}

func printHuman(stdout io.Writer, stderr io.Writer, op contractv1.OperationName, result any) int {
	switch op {
	case contractv1.OpContract:
		info := result.(contractv1.ContractInfo)
		fmt.Fprintf(stdout, "contract: %s\ncli: %s\nbinary: %s\n", info.Contract, info.CLI, info.BinaryVersion)
		fmt.Fprintln(stdout, "operations:")
		for _, operation := range info.Operations {
			fmt.Fprintf(stdout, "  %s (%s)\n", operation.Op, operation.Result)
		}
		return 0
	case contractv1.OpVersion:
		info := result.(contractv1.VersionInfo)
		fmt.Fprintf(stdout, "weave %s (contract 1)\n", info.BinaryVersion)
		return 0
	case contractv1.OpExtensions:
		list := result.(contractv1.ExtensionList)
		for _, extension := range list.Extensions {
			fmt.Fprintf(stdout, "%-24s %-8s %s\n", extension.Name, extension.Source, extension.Path)
		}
		return 0
	case contractv1.OpExport:
		report := result.(contractv1.ExportReport)
		if report.DryRun {
			fmt.Fprintln(stdout, "dry run: no files written")
		}
		fmt.Fprintf(stdout, "exported %d documents to %s (%s)\n", report.Documents, report.Out, report.Format)
		fmt.Fprintf(stdout, "encrypted=%d tokenized=%d hashed=%d\n", report.Encrypted, report.Tokenized, report.Hashed)
		return 0
	case contractv1.OpParse:
		switch value := result.(type) {
		case *contractv1.Bundle:
			version := "unknown"
			if value.OkfVersion != nil {
				version = string(*value.OkfVersion)
			}
			fmt.Fprintf(stdout, "%d concepts, okf_version %s, valid=%t\n", len(value.Concepts), version, value.Validation.Valid)
		case contractv1.SourceBundles:
			for _, source := range value.Sources {
				fmt.Fprintf(stdout, "%s: %d concepts, valid=%t\n", source.Source, len(source.Bundle.Concepts), source.Bundle.Validation.Valid)
			}
		}
		return 0
	case contractv1.OpGraph:
		switch value := result.(type) {
		case contractv1.Graph:
			fmt.Fprintf(stdout, "%d nodes, %d edges\n", len(value.Nodes), len(value.Edges))
		case contractv1.SourceGraphs:
			for _, source := range value.Sources {
				fmt.Fprintf(stdout, "%s: %d nodes, %d edges\n", source.Source, len(source.Graph.Nodes), len(source.Graph.Edges))
			}
		}
		return 0
	case contractv1.OpSearch:
		response := result.(contractv1.SearchResponse)
		if response.Mode == contractv1.SearchBM25 {
			if len(response.Lexical) == 0 {
				fmt.Fprintln(stderr, "weave: no matches")
				return 1
			}
			for _, item := range response.Lexical {
				fmt.Fprintf(stdout, "%s %s [%s]\n", strconv.FormatFloat(item.Score, 'f', -1, 64), item.Title, item.Path)
			}
			return 0
		}
		switch response.Mode {
		case contractv1.SearchKeyword:
			if len(response.Keyword) == 0 {
				fmt.Fprintln(stderr, "weave: no matches")
				return 1
			}
			for _, item := range response.Keyword {
				fmt.Fprintf(stdout, "%s %s [%s]\n", strconv.FormatFloat(item.Score, 'f', -1, 64), item.Title, strings.Join(item.MatchedKeywords, ", "))
			}
		case contractv1.SearchFuzzy:
			if len(response.Fuzzy) == 0 {
				fmt.Fprintln(stderr, "weave: no matches")
				return 1
			}
			for _, item := range response.Fuzzy {
				fmt.Fprintf(stdout, "%s %s [%s]\n", strconv.FormatFloat(item.Score, 'f', -1, 64), item.Title, fieldsText(item.MatchedFields))
			}
		default:
			if len(response.Hybrid) == 0 {
				fmt.Fprintln(stderr, "weave: no matches")
				return 1
			}
			for _, item := range response.Hybrid {
				fmt.Fprintf(stdout, "%s %s [%s]\n", strconv.FormatFloat(item.TotalScore, 'f', -1, 64), item.Title, item.Path)
			}
		}
		return 0
	case contractv1.OpList:
		results := result.([]contractv1.ListResult)
		if len(results) == 0 {
			fmt.Fprintln(stderr, "weave: no concepts match the given filters")
			fmt.Fprintln(stderr, "hint: adjust --type/--tag/--status or run `weave list <path>`")
			return 0
		}
		for _, item := range results {
			if item.Source != "" {
				fmt.Fprintf(stdout, "%s: %s [%s]\n", item.Source, item.Title, item.Path)
				continue
			}
			fmt.Fprintf(stdout, "%s [%s]\n", item.Title, item.Path)
		}
		return 0
	case contractv1.OpShow:
		show := result.(*contractv1.ShowResult)
		fmt.Fprintf(stdout, "# %s\nType: %s\n", titleOrID(show.Title, show.ID), show.Type)
		if show.Source != "" {
			fmt.Fprintf(stdout, "Source: %s\n", show.Source)
		}
		if show.Status != nil {
			fmt.Fprintf(stdout, "Status: %s\n", *show.Status)
		}
		if len(show.Tags) > 0 {
			fmt.Fprintf(stdout, "Tags: %s\n", strings.Join(show.Tags, ", "))
		}
		if show.Description != nil {
			fmt.Fprintf(stdout, "Description: %s\n", *show.Description)
		}
		fmt.Fprintf(stdout, "Backlinks: %d\nLinks:\n", len(show.Backlinks))
		if len(show.Links) == 0 {
			fmt.Fprintln(stdout, "- none")
		} else {
			for _, link := range show.Links {
				fmt.Fprintf(stdout, "- %s -> %s\n", link.Raw, link.Target)
			}
		}
		fmt.Fprintf(stdout, "\n%s", show.Body)
		return 0
	case contractv1.OpBacklinks:
		backlinks := result.(*contractv1.BacklinksResult)
		if len(backlinks.Backlinks) == 0 {
			fmt.Fprintf(stdout, "weave: no backlinks for %s\n", backlinks.ID)
			return 0
		}
		for _, entry := range backlinks.Backlinks {
			fmt.Fprintf(stdout, "- %s: %s\n", entry.From, entry.Title)
		}
		return 0
	case contractv1.OpPath:
		route := result.(*contractv1.PathResult)
		fmt.Fprintln(stdout, strings.Join(route.Path, " → "))
		return 0
	case contractv1.OpAudit:
		report := result.(contractv1.AuditReport)
		printAuditHuman(stdout, report)
		if audit.Failed(report) {
			return 1
		}
		return 0
	case contractv1.OpRelate:
		printRelateHuman(stdout, result.(contractv1.RelateResult))
		return 0
	case contractv1.OpExplain:
		printExplainHuman(stdout, result.(contractv1.ExplainResult))
		return 0
	case contractv1.OpIngest, contractv1.OpRebuild:
		report := result.(contractv1.IngestReport)
		if report.DryRun {
			fmt.Fprintln(stdout, "dry run: no changes written")
		}
		fmt.Fprintf(stdout, "%s: %d documents, %d edges (%s)\n",
			report.Bundle, report.Documents, report.Edges, report.Mode)
		fmt.Fprintf(stdout, "db: %s\n", report.DB)
		return 0
	case contractv1.OpSync:
		report := result.(contractv1.SyncReport)
		if report.DryRun {
			fmt.Fprintln(stdout, "dry run: no changes written")
		}
		fmt.Fprintf(stdout, "%s: %d documents (%d added, %d updated, %d removed, %d skipped)\n",
			report.Bundle, report.Documents, report.Added, report.Updated, report.Removed, report.Skipped)
		fmt.Fprintf(stdout, "db: %s (%s)\n", report.DB, report.Mode)
		return 0
	default:
		fmt.Fprintln(stderr, "weave: unsupported operation")
		return 1
	}
}

func printExplainHuman(stdout io.Writer, result contractv1.ExplainResult) {
	fmt.Fprintf(stdout, "query: %s\n", result.Query)
	fmt.Fprintf(stdout, "mode: %s (top %d)\n", result.Mode, result.TopK)
	fmt.Fprintf(stdout, "result: %s  rank %d\n", result.ID, result.Rank)

	fmt.Fprintf(stdout, "\nkeyword  %.3f\n", result.Keyword.Score)
	for _, token := range result.Keyword.Tokens {
		switch token.Match {
		case contractv1.ExplainMatchExact, contractv1.ExplainMatchPrefix:
			fmt.Fprintf(stdout, "  %s  %s  %.3f  → keyword %q\n",
				token.QueryToken, token.Match, token.Score, token.MatchedKeyword)
		default:
			fmt.Fprintf(stdout, "  %s  none  —  (no keyword match)\n", token.QueryToken)
		}
	}

	fmt.Fprintf(stdout, "\nfuzzy    %.3f\n", result.Fuzzy.Score)
	for _, field := range result.Fuzzy.Fields {
		fmt.Fprintf(stdout, "  %-12s %.3f  weight %.2f  contribution %.3f\n",
			field.Field, field.Score, field.Weight, field.Contribution)
	}
	if len(result.Fuzzy.Tokens) > 0 {
		fmt.Fprintln(stdout, "  token matches")
		for _, token := range result.Fuzzy.Tokens {
			fmt.Fprintf(stdout, "    %s → %s  %s  %.2f\n",
				token.QueryToken, token.MatchedToken, token.Field, token.Similarity)
		}
	}

	if result.Lexical != nil {
		fmt.Fprintf(stdout, "\nlexical  raw %.3f  normalized %.3f\n",
			result.Lexical.RawScore, result.Lexical.Score)
		for _, field := range result.Lexical.MatchedFields {
			fmt.Fprintf(stdout, "  %s  matched\n", field)
		}
		for _, token := range result.Lexical.Tokens {
			fmt.Fprintf(stdout, "  %s  contribution %.3f\n", token.QueryToken, token.Contribution)
		}
	}

	fmt.Fprintf(stdout, "\nhybrid   %.3f\n", result.Hybrid.Total)
	if result.Hybrid.LexicalWeight > 0 {
		fmt.Fprintf(stdout, "  %.2f * %.3f (lexical) + %.2f * %.3f (fuzzy)\n",
			result.Hybrid.LexicalWeight, lexicalScore(result),
			result.Hybrid.FuzzyWeight, result.Fuzzy.Score)
	} else {
		fmt.Fprintf(stdout, "  %.2f * %.3f + %.2f * %.3f\n",
			result.Hybrid.KeywordWeight, result.Keyword.Score,
			result.Hybrid.FuzzyWeight, result.Fuzzy.Score)
	}
}

func lexicalScore(result contractv1.ExplainResult) float64 {
	if result.Lexical == nil {
		return 0
	}
	return result.Lexical.Score
}

func printRelateHuman(stdout io.Writer, result contractv1.RelateResult) {
	hopWord := "hops"
	if result.Depth == 1 {
		hopWord = "hop"
	}
	fmt.Fprintf(stdout, "%s  (%d %s, %s)\n", result.Root, result.Depth, hopWord, result.Direction)
	if len(result.Edges) == 0 {
		return
	}

	if result.Depth <= 1 {
		printRelateDepthOne(stdout, result)
		return
	}
	for hop := 1; hop <= result.Depth; hop++ {
		printed := false
		for _, edge := range result.Edges {
			if edge.Depth != hop {
				continue
			}
			if !printed {
				fmt.Fprintf(stdout, "hop %d\n", hop)
				printed = true
			}
			fmt.Fprintf(stdout, "  %s → %s (%s)\n", edge.From, edge.To, edge.Source)
		}
	}
}

func printRelateDepthOne(stdout io.Writer, result contractv1.RelateResult) {
	outbound := make([]contractv1.RelateEdge, 0)
	inbound := make([]contractv1.RelateEdge, 0)
	for _, edge := range result.Edges {
		switch {
		case edge.From == result.Root:
			outbound = append(outbound, edge)
		case edge.To == result.Root:
			inbound = append(inbound, edge)
		}
	}

	if len(outbound) > 0 {
		fmt.Fprintln(stdout, "  outbound")
		for _, edge := range outbound {
			fmt.Fprintf(stdout, "    %s → %s\n", edge.Source, edge.To)
		}
	}
	if len(inbound) > 0 {
		fmt.Fprintln(stdout, "  inbound")
		for _, edge := range inbound {
			fmt.Fprintf(stdout, "    %s\n", edge.From)
		}
	}
}

func printAuditHuman(stdout io.Writer, report contractv1.AuditReport) {
	fmt.Fprintf(stdout, "weave audit: %s\n", report.Path)
	fmt.Fprintf(stdout, "  %d concepts · %d edges · %d orphans\n",
		report.Summary.Concepts, report.Summary.Edges, report.Summary.Orphans)
	fmt.Fprintf(stdout, "  %d missing keywords · %d broken links\n",
		report.Summary.MissingKeywords, report.Summary.BrokenLinks)

	errors, warnings, _ := countSeverities(report.Findings)
	printAuditSection(stdout, "errors", contractv1.AuditSeverityError, report.Findings)
	printAuditSection(stdout, "warnings", contractv1.AuditSeverityWarning, report.Findings)
	printAuditSection(stdout, "info", contractv1.AuditSeverityInfo, report.Findings)

	if audit.Failed(report) {
		fmt.Fprintf(stdout, "\naudit: failed (%d errors, %d warnings)\n", errors, warnings)
		return
	}
	fmt.Fprintln(stdout, "\naudit: ok")
}

func printAuditSection(
	stdout io.Writer,
	title string,
	severity contractv1.AuditSeverity,
	findings []contractv1.AuditFinding,
) {
	printedHeader := false
	for _, finding := range findings {
		if finding.Severity != severity {
			continue
		}
		if !printedHeader {
			fmt.Fprintf(stdout, "\n%s\n", title)
			printedHeader = true
		}
		key := finding.Concept
		if key == "" {
			key = finding.Source
		}
		if key == "" {
			key = "-"
		}
		detail := finding.Target
		if detail == "" {
			detail = finding.Message
		}
		fmt.Fprintf(stdout, "  %s  %s  %s\n", key, finding.Code, detail)
	}
}

func countSeverities(findings []contractv1.AuditFinding) (int, int, int) {
	errors, warnings, infos := 0, 0, 0
	for _, finding := range findings {
		switch finding.Severity {
		case contractv1.AuditSeverityError:
			errors++
		case contractv1.AuditSeverityWarning:
			warnings++
		case contractv1.AuditSeverityInfo:
			infos++
		}
	}
	return errors, warnings, infos
}

func fieldsText(fields []contractv1.SearchField) string {
	values := make([]string, len(fields))
	for index, field := range fields {
		values[index] = string(field)
	}
	return strings.Join(values, ", ")
}

func titleOrID(title *string, id string) string {
	if title == nil {
		return id
	}
	return *title
}
