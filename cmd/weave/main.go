package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
	"github.com/prulloac/weave/engine"
	"github.com/prulloac/weave/engine/audit"
	"github.com/prulloac/weave/engine/compression"
	"github.com/prulloac/weave/engine/operations"
	"github.com/prulloac/weave/engine/privacy"
)

var version = "dev"

const contractVersion = "weave.contract/v1"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	if len(args) == 0 {
		printHelp(stdout)
		return 0
	}

	if args[0] == "--help" || args[0] == "-h" || args[0] == "help" {
		printHelp(stdout)
		return 0
	}
	if args[0] == "--version" || args[0] == "-v" {
		fmt.Fprintf(stdout, "weave %s (contract 1)\n", version)
		return 0
	}

	command := args[0]
	if command == "mcp" {
		return runMCP(args[1:], stdout, stderr)
	}
	jsonMode := contains(args[1:], "--json")
	flags, err := parseFlags(args[1:])
	if err != nil {
		return usageError(stdout, stderr, jsonMode || flags.json, err.Error())
	}
	if flags.help {
		printCommandHelp(stdout, command)
		return 0
	}

	request, exitCode, err := buildRequest(command, flags)
	if err != nil {
		return usageError(stdout, stderr, flags.json, err.Error())
	}
	if exitCode != 0 {
		return exitCode
	}
	request.DB = flags.db
	request.NoDB = flags.noDB
	request.Strict = flags.strict
	request.Permissive = flags.permissive
	request.Codec = flags.codec
	request.Decrypt = flags.decrypt

	if !flags.json && request.Path != "" {
		if warning := privacy.SecurityFileWarning(request.Path); warning != "" {
			fmt.Fprintf(stderr, "weave: %s\n", warning)
		}
	}

	executor := operations.New(version, engine.Options{Cache: !flags.noCache})
	value := executor.Execute(context.Background(), request)

	switch envelope := value.(type) {
	case contractv1.SuccessEnvelope:
		if flags.json {
			if err := writeJSON(stdout, envelope); err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
				return 1
			}
			if exit := successExitCode(request.Op, envelope.Result); exit != 0 {
				return exit
			}
			if isEmptyResult(request.Op, envelope.Result) {
				fmt.Fprintln(stderr, "weave: no matches")
				return 1
			}
			return 0
		}
		return printHuman(stdout, stderr, request.Op, envelope.Result)
	case contractv1.FailureEnvelope:
		if flags.json {
			if err := writeJSON(stdout, envelope); err != nil {
				fmt.Fprintf(stderr, "weave: %v\n", err)
			}
		}
		fmt.Fprintf(stderr, "weave: %s\n", envelope.Error.Message)
		return exitCodeFor(envelope.Error.Code)
	default:
		fmt.Fprintln(stderr, "weave: internal error")
		return 1
	}
}

type flags struct {
	json        bool
	noCache     bool
	help        bool
	limit       *int
	topK        *int
	mode        contractv1.SearchMode
	types       []string
	tags        []string
	status      string
	failOn      contractv1.AuditFailOn
	verifyKeys  bool
	decrypt     bool
	depth       *int
	direction   contractv1.RelateDirection
	id          string
	db          string
	noDB        bool
	strict      bool
	permissive  bool
	codec       string
	positionals []string
}

func parseFlags(args []string) (flags, error) {
	var result flags

	for index := 0; index < len(args); index++ {
		arg := args[index]
		switch arg {
		case "--json":
			result.json = true
		case "--no-cache":
			result.noCache = true
		case "--help", "-h":
			result.help = true
		case "--limit":
			value, next, err := flagValue(args, index, "--limit")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 {
				return result, fmt.Errorf("--limit requires a non-negative number")
			}
			result.limit = &parsed
		case "--type":
			value, next, err := flagValue(args, index, "--type")
			if err != nil {
				return result, err
			}
			index = next
			result.types = append(result.types, value)
		case "--tag":
			value, next, err := flagValue(args, index, "--tag")
			if err != nil {
				return result, err
			}
			index = next
			result.tags = append(result.tags, value)
		case "--status":
			value, next, err := flagValue(args, index, "--status")
			if err != nil {
				return result, err
			}
			index = next
			result.status = value
		case "--mode":
			value, next, err := flagValue(args, index, "--mode")
			if err != nil {
				return result, err
			}
			index = next
			mode := contractv1.SearchMode(value)
			switch mode {
			case contractv1.SearchKeyword, contractv1.SearchFuzzy, contractv1.SearchHybrid, contractv1.SearchBM25:
				result.mode = mode
			default:
				return result, fmt.Errorf("--mode must be hybrid, keyword, fuzzy or bm25")
			}
		case "--top-k":
			value, next, err := flagValue(args, index, "--top-k")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 1 || parsed > 100 {
				return result, fmt.Errorf("--top-k must be between 1 and 100")
			}
			result.topK = &parsed
		case "--fail-on":
			value, next, err := flagValue(args, index, "--fail-on")
			if err != nil {
				return result, err
			}
			index = next
			failOn := contractv1.AuditFailOn(value)
			switch failOn {
			case contractv1.AuditFailOnError, contractv1.AuditFailOnWarning, contractv1.AuditFailOnNone:
				result.failOn = failOn
			default:
				return result, fmt.Errorf("--fail-on must be error, warning or none")
			}
		case "--verify-keys":
			result.verifyKeys = true
		case "--decrypt":
			result.decrypt = true
		case "--id":
			value, next, err := flagValue(args, index, "--id")
			if err != nil {
				return result, err
			}
			index = next
			result.id = value
		case "--db":
			value, next, err := flagValue(args, index, "--db")
			if err != nil {
				return result, err
			}
			index = next
			result.db = value
		case "--no-db":
			result.noDB = true
		case "--strict":
			result.strict = true
		case "--permissive":
			result.permissive = true
		case "--codec":
			value, next, err := flagValue(args, index, "--codec")
			if err != nil {
				return result, err
			}
			index = next
			if !compression.Valid(value) {
				return result, fmt.Errorf("--codec must be zstd, lz4 or none")
			}
			result.codec = value
		case "--depth":
			value, next, err := flagValue(args, index, "--depth")
			if err != nil {
				return result, err
			}
			index = next
			parsed, err := strconv.Atoi(value)
			if err != nil || parsed < 0 || parsed > 5 {
				return result, fmt.Errorf("--depth must be between 0 and 5")
			}
			result.depth = &parsed
		case "--direction":
			value, next, err := flagValue(args, index, "--direction")
			if err != nil {
				return result, err
			}
			index = next
			direction := contractv1.RelateDirection(value)
			switch direction {
			case contractv1.RelateOut, contractv1.RelateIn, contractv1.RelateBoth:
				result.direction = direction
			default:
				return result, fmt.Errorf("--direction must be out, in or both")
			}
		case "--":
			result.positionals = append(result.positionals, args[index+1:]...)
			return result, nil
		default:
			if strings.HasPrefix(arg, "-") {
				return result, fmt.Errorf("unknown option: %s", arg)
			}
			result.positionals = append(result.positionals, arg)
		}
	}

	if result.db != "" && result.noDB {
		return result, fmt.Errorf("--db and --no-db are mutually exclusive")
	}
	if result.strict && result.permissive {
		return result, fmt.Errorf("--strict and --permissive are mutually exclusive")
	}
	return result, nil
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func flagValue(args []string, index int, name string) (string, int, error) {
	if index+1 >= len(args) {
		return "", index, fmt.Errorf("%s requires a value", name)
	}
	return args[index+1], index + 1, nil
}

func buildRequest(command string, flags flags) (contractv1.OperationRequest, int, error) {
	switch command {
	case "contract", "version":
		if len(flags.positionals) != 0 {
			return contractv1.OperationRequest{}, 0, fmt.Errorf("%s does not take positional arguments", command)
		}
		op := contractv1.OpContract
		if command == "version" {
			op = contractv1.OpVersion
		}
		return contractv1.OperationRequest{Op: op}, 0, nil
	case "parse":
		return contractv1.OperationRequest{Op: contractv1.OpParse, Path: flags.positional(0)}, 0, requirePositional(command, flags, 1)
	case "graph":
		return contractv1.OperationRequest{Op: contractv1.OpGraph, Path: flags.positional(0)}, 0, requirePositional(command, flags, 1)
	case "search":
		if err := requirePositional(command, flags, 2); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:    contractv1.OpSearch,
			Path:  flags.positional(0),
			Query: flags.positional(1),
			Mode:  flags.mode,
			TopK:  flags.topK,
		}, 0, nil
	case "list":
		if err := requirePositional(command, flags, 1); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:     contractv1.OpList,
			Path:   flags.positional(0),
			Types:  flags.types,
			Tags:   flags.tags,
			Status: flags.status,
			Limit:  flags.limit,
		}, 0, nil
	case "show", "backlinks":
		if err := requirePositional(command, flags, 2); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		op := contractv1.OpShow
		if command == "backlinks" {
			op = contractv1.OpBacklinks
		}
		return contractv1.OperationRequest{
			Op:     op,
			Path:   flags.positional(0),
			Target: flags.positional(1),
		}, 0, nil
	case "path":
		if err := requirePositional(command, flags, 3); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:   contractv1.OpPath,
			Path: flags.positional(0),
			From: flags.positional(1),
			To:   flags.positional(2),
		}, 0, nil
	case "audit":
		if err := requirePositional(command, flags, 1); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		failOn := flags.failOn
		if failOn == "" {
			failOn = contractv1.AuditFailOnError
		}
		return contractv1.OperationRequest{
			Op:         contractv1.OpAudit,
			Path:       flags.positional(0),
			FailOn:     failOn,
			VerifyKeys: flags.verifyKeys,
		}, 0, nil
	case "relate":
		if err := requirePositional(command, flags, 2); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		depth := flags.depth
		if depth == nil {
			value := 1
			depth = &value
		}
		direction := flags.direction
		if direction == "" {
			direction = contractv1.RelateBoth
		}
		return contractv1.OperationRequest{
			Op:        contractv1.OpRelate,
			Path:      flags.positional(0),
			Target:    flags.positional(1),
			Depth:     depth,
			Direction: direction,
		}, 0, nil
	case "explain":
		if err := requirePositional(command, flags, 2); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:     contractv1.OpExplain,
			Path:   flags.positional(0),
			Query:  flags.positional(1),
			Target: flags.id,
			Mode:   flags.mode,
			TopK:   flags.topK,
		}, 0, nil
	case "ingest", "rebuild":
		if err := requirePositional(command, flags, 1); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		op := contractv1.OpIngest
		if command == "rebuild" {
			op = contractv1.OpRebuild
		}
		return contractv1.OperationRequest{
			Op:   op,
			Path: flags.positional(0),
		}, 0, nil
	case "sync":
		if err := requirePositional(command, flags, 1); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:   contractv1.OpSync,
			Path: flags.positional(0),
		}, 0, nil
	case "mount", "unmount", "status":
		return contractv1.OperationRequest{}, 2, fmt.Errorf("%s is not implemented in the Go CLI yet", command)
	default:
		return contractv1.OperationRequest{}, 2, fmt.Errorf("unknown command: %s", command)
	}
}

func (f flags) positional(index int) string {
	if index < 0 || index >= len(f.positionals) {
		return ""
	}
	return f.positionals[index]
}

func requirePositional(command string, f flags, count int) error {
	if len(f.positionals) != count {
		return fmt.Errorf("%s requires %d positional argument(s)", command, count)
	}
	return nil
}

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
	fmt.Fprintf(stderr, "weave: %s\n", message)
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

func exitCodeFor(code contractv1.ErrorCode) int {
	switch code {
	case contractv1.ErrInvalidRequest, contractv1.ErrInvalidRoot, contractv1.ErrPrivacyDirective:
		return 2
	case contractv1.ErrPermissionDenied, contractv1.ErrPrivacyKey,
		contractv1.ErrPrivacyProvider, contractv1.ErrPrivacyDecrypt:
		return 3
	default:
		return 1
	}
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
	case contractv1.OpList:
		values, ok := result.([]contractv1.ListResult)
		return ok && len(values) == 0
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
	case contractv1.OpParse:
		bundle := result.(*contractv1.Bundle)
		version := "unknown"
		if bundle.OkfVersion != nil {
			version = string(*bundle.OkfVersion)
		}
		fmt.Fprintf(stdout, "%d concepts, okf_version %s, valid=%t\n", len(bundle.Concepts), version, bundle.Validation.Valid)
		return 0
	case contractv1.OpGraph:
		graph := result.(contractv1.Graph)
		fmt.Fprintf(stdout, "%d nodes, %d edges\n", len(graph.Nodes), len(graph.Edges))
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
			return 1
		}
		for _, item := range results {
			fmt.Fprintf(stdout, "%s [%s]\n", item.Title, item.Path)
		}
		return 0
	case contractv1.OpShow:
		show := result.(*contractv1.ShowResult)
		fmt.Fprintf(stdout, "# %s\nType: %s\n", titleOrID(show.Title, show.ID), show.Type)
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
		fmt.Fprintf(stdout, "%s: %d documents, %d edges (%s)\n",
			report.Bundle, report.Documents, report.Edges, report.Mode)
		fmt.Fprintf(stdout, "db: %s\n", report.DB)
		return 0
	case contractv1.OpSync:
		report := result.(contractv1.SyncReport)
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

func printHelp(writer io.Writer) {
	fmt.Fprintf(writer, `weave — mount OKF bundles locally

usage:
  weave contract [--json]
  weave parse <path> [--json] [--no-cache]
  weave graph <path> [--json] [--no-cache]
  weave search <path> <query> [--mode hybrid|keyword|fuzzy|bm25] [--top-k <n>] [--json] [--no-cache]
  weave list <path> [--type <t>] [--tag <t>] [--status <s>] [--limit <n>] [--json] [--no-cache]
  weave show <path> <id|file> [--decrypt] [--json] [--no-cache]
  weave backlinks <path> <id|file> [--json] [--no-cache]
  weave path <path> <id|file> <id|file> [--json] [--no-cache]
  weave audit <path> [--fail-on error|warning|none] [--verify-keys] [--json] [--no-cache]
  weave relate <path> <id|file> [--depth <n>] [--direction out|in|both] [--json] [--no-cache]
  weave explain <path> <query> [--id <id>] [--mode hybrid|keyword|fuzzy] [--top-k <n>] [--json] [--no-cache]
  weave ingest <path> [--db <file>] [--strict|--permissive] [--codec zstd|lz4|none] [--json] [--no-cache]
  weave rebuild <path> [--db <file>] [--strict|--permissive] [--codec zstd|lz4|none] [--json] [--no-cache]
  weave sync <path> [--db <file>] [--strict|--permissive] [--codec zstd|lz4|none] [--json] [--no-cache]
  weave mcp --bundle <path> [--db <file>] [--stdio | --listen <addr>] [--token <token>]
  weave version [--json]
  weave --version | --help

contract: %s
`, contractVersion)
}

func printCommandHelp(writer io.Writer, command string) {
	fmt.Fprintf(writer, "weave %s: see `weave --help` for usage\n", command)
}
