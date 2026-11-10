package main

import (
	"fmt"
	"math"
	"strings"

	contractv1 "github.com/prulloac/weave/contract/v1"
)

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
		relations, err := relationKinds(flags.relations)
		if err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{Op: contractv1.OpGraph, Path: flags.positional(0), Relations: relations}, 0, requirePositional(command, flags, 1)
	case "search":
		if err := requirePositional(command, flags, 2); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		request := contractv1.OperationRequest{
			Op:    contractv1.OpSearch,
			Path:  flags.positional(0),
			Query: flags.positional(1),
			Mode:  flags.mode,
			TopK:  flags.topK,
		}
		if err := scorerRequest(&request, flags); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return request, 0, nil
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
		relations, err := relationKinds(flags.relations)
		if err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:        contractv1.OpPath,
			Path:      flags.positional(0),
			From:      flags.positional(1),
			To:        flags.positional(2),
			Relations: relations,
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
		relations, err := relationKinds(flags.relations)
		if err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:        contractv1.OpRelate,
			Path:      flags.positional(0),
			Target:    flags.positional(1),
			Depth:     depth,
			Direction: direction,
			Relations: relations,
		}, 0, nil
	case "explain":
		if err := requirePositional(command, flags, 2); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		request := contractv1.OperationRequest{
			Op:     contractv1.OpExplain,
			Path:   flags.positional(0),
			Query:  flags.positional(1),
			Target: flags.id,
			Mode:   flags.mode,
			TopK:   flags.topK,
		}
		if err := scorerRequest(&request, flags); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return request, 0, nil
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
	case "extensions":
		if len(flags.positionals) < 1 || flags.positionals[0] != "list" {
			return contractv1.OperationRequest{}, 0, fmt.Errorf("extensions requires the list subcommand")
		}
		if len(flags.positionals) > 2 {
			return contractv1.OperationRequest{}, 0, fmt.Errorf("extensions list accepts at most one path")
		}
		path := ""
		if len(flags.positionals) == 2 {
			path = flags.positionals[1]
		}
		return contractv1.OperationRequest{
			Op:   contractv1.OpExtensions,
			Path: path,
		}, 0, nil
	case "codecs":
		if len(flags.positionals) < 1 || flags.positionals[0] != "list" {
			return contractv1.OperationRequest{}, 0, fmt.Errorf("codecs requires the list subcommand")
		}
		if len(flags.positionals) > 2 {
			return contractv1.OperationRequest{}, 0, fmt.Errorf("codecs list accepts at most one path")
		}
		path := ""
		if len(flags.positionals) == 2 {
			path = flags.positionals[1]
		}
		return contractv1.OperationRequest{Op: contractv1.OpCodecs, Path: path}, 0, nil
	case "export":
		if err := requirePositional(command, flags, 1); err != nil {
			return contractv1.OperationRequest{}, 0, err
		}
		return contractv1.OperationRequest{
			Op:   contractv1.OpExport,
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

func relationKinds(values []string) ([]contractv1.RelationKind, error) {
	if len(values) == 0 {
		return nil, nil
	}
	result := make([]contractv1.RelationKind, 0, len(values))
	for _, value := range values {
		kind := contractv1.RelationKind(strings.TrimSpace(value))
		switch kind {
		case contractv1.RelationLink, contractv1.RelationRelated, contractv1.RelationImports,
			contractv1.RelationDeclares, contractv1.RelationReferences, contractv1.RelationCalls,
			contractv1.RelationInherits, contractv1.RelationImplements:
			result = append(result, kind)
		default:
			return nil, fmt.Errorf("unknown relation kind %q", value)
		}
	}
	return result, nil
}

func scorerRequest(request *contractv1.OperationRequest, f flags) error {
	if f.scorer == "" {
		if f.scorerWeight != nil || f.candidateK != nil {
			return fmt.Errorf("--scorer-weight and --candidate-k require --scorer")
		}
		return nil
	}
	if f.rerank != "" {
		return fmt.Errorf("--scorer and --rerank are mutually exclusive")
	}
	if request.Mode == contractv1.SearchSemantic {
		return fmt.Errorf("--scorer is not supported with --mode semantic")
	}
	topK := 10
	if f.topK != nil {
		topK = *f.topK
	}
	request.Scorer = f.scorer
	if f.scorerWeight != nil {
		weight := *f.scorerWeight
		if math.IsNaN(weight) || math.IsInf(weight, 0) || weight < 0 || weight > 1 {
			return fmt.Errorf("--scorer-weight must be between 0 and 1")
		}
		request.ScorerWeight = &weight
	}
	if f.candidateK != nil {
		candidateK := *f.candidateK
		if candidateK < 1 || candidateK > 100 {
			return fmt.Errorf("--candidate-k must be between 1 and 100")
		}
		if candidateK < topK {
			return fmt.Errorf("--candidate-k must be at least --top-k (%d)", topK)
		}
		request.CandidateK = &candidateK
	}
	return nil
}
