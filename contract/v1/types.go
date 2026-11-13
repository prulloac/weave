// Package contractv1 contains the hand-written Go representation of the
// language-neutral Weave contract v1. The JSON Schemas under schema/ are
// authoritative; these structs are validated against them in CI.
package contractv1

type OkfVersion string

const (
	OkfVersion02 OkfVersion = "0.2"
)

type ConceptStatus string

const (
	StatusDraft      ConceptStatus = "draft"
	StatusStable     ConceptStatus = "stable"
	StatusDeprecated ConceptStatus = "deprecated"
)

type SearchMode string

const (
	SearchKeyword  SearchMode = "keyword"
	SearchFuzzy    SearchMode = "fuzzy"
	SearchHybrid   SearchMode = "hybrid"
	SearchBM25     SearchMode = "bm25"
	SearchSemantic SearchMode = "semantic"
)

type SearchField string

const (
	FieldTitle       SearchField = "title"
	FieldKeywords    SearchField = "keywords"
	FieldDescription SearchField = "description"
	FieldBody        SearchField = "body"
)

type AuditFailOn string

const (
	AuditFailOnError   AuditFailOn = "error"
	AuditFailOnWarning AuditFailOn = "warning"
	AuditFailOnNone    AuditFailOn = "none"
)

type AuditSeverity string

const (
	AuditSeverityError   AuditSeverity = "error"
	AuditSeverityWarning AuditSeverity = "warning"
	AuditSeverityInfo    AuditSeverity = "info"
)

type AuditFindingCode string

const (
	AuditMissingFrontmatter     AuditFindingCode = "MISSING_FRONTMATTER"
	AuditMissingType            AuditFindingCode = "MISSING_TYPE"
	AuditMissingKeywords        AuditFindingCode = "MISSING_KEYWORDS"
	AuditBrokenLink             AuditFindingCode = "BROKEN_LINK"
	AuditBrokenRelated          AuditFindingCode = "BROKEN_RELATED"
	AuditMissingIndexTarget     AuditFindingCode = "MISSING_INDEX_TARGET"
	AuditUnindexedConcept       AuditFindingCode = "UNINDEXED_CONCEPT"
	AuditOrphanConcept          AuditFindingCode = "ORPHAN_CONCEPT"
	AuditUnsupportedOKFVersion  AuditFindingCode = "UNSUPPORTED_OKF_VERSION"
	AuditEmptyBundle            AuditFindingCode = "EMPTY_BUNDLE"
	AuditDanglingExternal       AuditFindingCode = "DANGLING_EXTERNAL"
	AuditKeyUnavailable         AuditFindingCode = "KEY_UNAVAILABLE"
	AuditCodecMissing           AuditFindingCode = "CODEC_MISSING"
	AuditEmbeddingStale         AuditFindingCode = "EMBEDDING_STALE"
	AuditEmbeddingIndexMismatch AuditFindingCode = "EMBEDDING_INDEX_MISMATCH"
)

type RelateDirection string

const (
	RelateOut  RelateDirection = "out"
	RelateIn   RelateDirection = "in"
	RelateBoth RelateDirection = "both"
)

type ErrorCode string

const (
	ErrInvalidRequest        ErrorCode = "INVALID_REQUEST"
	ErrInvalidRoot           ErrorCode = "INVALID_ROOT"
	ErrPermissionDenied      ErrorCode = "PERMISSION_DENIED"
	ErrParseError            ErrorCode = "PARSE_ERROR"
	ErrConceptNotFound       ErrorCode = "CONCEPT_NOT_FOUND"
	ErrPathNotFound          ErrorCode = "PATH_NOT_FOUND"
	ErrBundleNotIndexed      ErrorCode = "BUNDLE_NOT_INDEXED"
	ErrPrivacyDirective      ErrorCode = "PRIVACY_DIRECTIVE_INVALID"
	ErrPrivacyKey            ErrorCode = "PRIVACY_KEY_UNAVAILABLE"
	ErrPrivacyProvider       ErrorCode = "PRIVACY_PROVIDER_UNSUPPORTED"
	ErrPrivacyDecrypt        ErrorCode = "PRIVACY_DECRYPT_FAILED"
	ErrConfirmation          ErrorCode = "CONFIRMATION_REQUIRED"
	ErrExtensionMissing      ErrorCode = "EXTENSION_NOT_FOUND"
	ErrExtensionTimeout      ErrorCode = "EXTENSION_TIMEOUT"
	ErrExtensionFailed       ErrorCode = "EXTENSION_FAILED"
	ErrExtensionProto        ErrorCode = "EXTENSION_PROTOCOL"
	ErrExtensionDataInvalid  ErrorCode = "EXTENSION_DATA_INVALID"
	ErrExtensionDataTooLarge ErrorCode = "EXTENSION_DATA_TOO_LARGE"
	ErrExtensionDataQuota    ErrorCode = "EXTENSION_DATA_QUOTA"
	ErrExtensionDataNotFound ErrorCode = "EXTENSION_DATA_NOT_FOUND"
	ErrAssetTypesRequired    ErrorCode = "ASSET_TYPES_REQUIRED"
	ErrBlobsRequireDB        ErrorCode = "BLOBS_REQUIRE_DB"
	ErrCodecNotFound         ErrorCode = "CODEC_NOT_FOUND"
	ErrCodecUnavailable      ErrorCode = "CODEC_UNAVAILABLE"
	ErrCodecFailed           ErrorCode = "CODEC_FAILED"
	ErrCodecTimeout          ErrorCode = "CODEC_TIMEOUT"
	ErrCodecProtocol         ErrorCode = "CODEC_PROTOCOL"
	ErrBodyDecode            ErrorCode = "BODY_DECODE_FAILED"
	ErrInternal              ErrorCode = "INTERNAL"
)

type OperationName string

const (
	OpContract   OperationName = "contract"
	OpParse      OperationName = "parse"
	OpGraph      OperationName = "graph"
	OpSearch     OperationName = "search"
	OpList       OperationName = "list"
	OpShow       OperationName = "show"
	OpBacklinks  OperationName = "backlinks"
	OpPath       OperationName = "path"
	OpVersion    OperationName = "version"
	OpAudit      OperationName = "audit"
	OpRelate     OperationName = "relate"
	OpExplain    OperationName = "explain"
	OpIngest     OperationName = "ingest"
	OpRebuild    OperationName = "rebuild"
	OpSync       OperationName = "sync"
	OpExtensions OperationName = "extensions"
	OpExport     OperationName = "export"
	OpCodecs     OperationName = "codecs"
)

// OkfLink is a single Markdown link extracted from a concept body.
type OkfLink struct {
	Raw              string  `json:"raw"`
	Target           string  `json:"target"`
	Resolved         *string `json:"resolved,omitempty"`
	ResolvesInBundle bool    `json:"resolves_in_bundle"`
}

// RelationKind is one graph relation kind.
type RelationKind string

const (
	RelationLink       RelationKind = "link"
	RelationRelated    RelationKind = "related"
	RelationImports    RelationKind = "imports"
	RelationDeclares   RelationKind = "declares"
	RelationReferences RelationKind = "references"
	RelationCalls      RelationKind = "calls"
	RelationInherits   RelationKind = "inherits"
	RelationImplements RelationKind = "implements"
	RelationContains   RelationKind = "contains"
	RelationDependsOn  RelationKind = "depends_on"
)

// ConceptRelation is one extension-provided relation from this concept to another.
type ConceptRelation struct {
	To   string       `json:"to"`
	Kind RelationKind `json:"kind"`
	Raw  *string      `json:"raw,omitempty"`
}

// OkfConcept is one parsed OKF concept document.
type OkfConcept struct {
	ID              string            `json:"id"`
	Path            string            `json:"path"`
	Frontmatter     map[string]any    `json:"frontmatter"`
	Type            string            `json:"type"`
	Keywords        []string          `json:"keywords"`
	Title           *string           `json:"title,omitempty"`
	Description     *string           `json:"description,omitempty"`
	Tags            []string          `json:"tags,omitempty"`
	Status          *ConceptStatus    `json:"status,omitempty"`
	Body            string            `json:"body"`
	Links           []OkfLink         `json:"links"`
	Relations       []ConceptRelation `json:"relations,omitempty"`
	Privacy         *PrivacyDirective `json:"privacy,omitempty"`
	ContentKind     string            `json:"content_kind,omitempty"`
	MediaType       string            `json:"media_type,omitempty"`
	ContentMetadata map[string]any    `json:"content_metadata,omitempty"`
	BodyBytes       []byte            `json:"-"`
	BodyText        bool              `json:"-"`
	BodyForm        string            `json:"-"`
	BodyCodec       string            `json:"-"`
	Size            int64             `json:"-"`
	StoredSize      int64             `json:"-"`
	ContentHash     string            `json:"-"`
}

// IndexEntry is one root index.md listing entry.
type IndexEntry struct {
	Title       string  `json:"title"`
	Target      string  `json:"target"`
	Description *string `json:"description,omitempty"`
}

// ConceptValidation records per-concept conformance.
type ConceptValidation struct {
	ID             string   `json:"id"`
	HasFrontmatter bool     `json:"has_frontmatter"`
	HasType        bool     `json:"has_type"`
	HasKeywords    bool     `json:"has_keywords"`
	Warnings       []string `json:"warnings"`
}

// Validation records bundle-level conformance.
type Validation struct {
	Valid      bool                `json:"valid"`
	Permissive bool                `json:"permissive,omitempty"`
	Concepts   []ConceptValidation `json:"concepts"`
	Warnings   []string            `json:"warnings"`
}

// Bundle is the parsed OKF bundle exposed by `weave parse`.
type Bundle struct {
	Root       string       `json:"root"`
	OkfVersion *OkfVersion  `json:"okf_version,omitempty"`
	Concepts   []OkfConcept `json:"concepts"`
	Index      []IndexEntry `json:"index,omitempty"`
	Validation Validation   `json:"validation"`
}

// GraphNode is a concept node with degree metadata.
type GraphNode struct {
	ID        string         `json:"id"`
	Path      string         `json:"path"`
	Type      string         `json:"type"`
	Source    string         `json:"source,omitempty"`
	Title     string         `json:"title"`
	Keywords  []string       `json:"keywords"`
	Status    *ConceptStatus `json:"status,omitempty"`
	Tags      []string       `json:"tags,omitempty"`
	InDegree  int            `json:"in_degree"`
	OutDegree int            `json:"out_degree"`
}

// GraphEdge is a directed edge between concepts.
type GraphEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Source string  `json:"source"`
	Raw    *string `json:"raw,omitempty"`
}

// Graph is the full graph view, including dangling targets.
type Graph struct {
	Nodes    []GraphNode          `json:"nodes"`
	Edges    []GraphEdge          `json:"edges"`
	Dangling map[string][]OkfLink `json:"dangling"`
}

// PrivacyPolicy is one declared privacy treatment.
type PrivacyPolicy string

const (
	PrivacyNone     PrivacyPolicy = "none"
	PrivacyEncrypt  PrivacyPolicy = "encrypt"
	PrivacyTokenize PrivacyPolicy = "tokenize"
	PrivacyHash     PrivacyPolicy = "hash"
	PrivacyMixed    PrivacyPolicy = "mixed"
)

// PrivacyDirective is one declared policy from frontmatter or an inline comment.
type PrivacyDirective struct {
	Policy       PrivacyPolicy `json:"policy"`
	Provider     string        `json:"provider,omitempty"`
	KeyID        string        `json:"key_id,omitempty"`
	ProviderName string        `json:"provider_name,omitempty"`
	KeyVersion   int           `json:"key_version,omitempty"`
	Encoded      bool          `json:"encoded,omitempty"`
}

// PrivacyState is the stored privacy summary for one concept.
type PrivacyState struct {
	Policy    PrivacyPolicy `json:"policy"`
	Provider  string        `json:"provider,omitempty"`
	KeyID     string        `json:"key_id,omitempty"`
	Redacted  bool          `json:"redacted"`
	Decrypted bool          `json:"decrypted"`
}

// KeywordHit is a keyword-dimension search result.
type KeywordHit struct {
	ID              string   `json:"id"`
	Title           string   `json:"title"`
	Path            string   `json:"path"`
	Type            string   `json:"type"`
	Source          string   `json:"source,omitempty"`
	Keywords        []string `json:"keywords"`
	MatchedKeywords []string `json:"matched_keywords"`
	Score           float64  `json:"score"`
	RerankScore     *float64 `json:"rerank_score,omitempty"`
	ScorerScore     *float64 `json:"scorer_score,omitempty"`
}

// FuzzyHit is a fuzzy-dimension search result.
type FuzzyHit struct {
	ID            string        `json:"id"`
	Title         string        `json:"title"`
	Path          string        `json:"path"`
	Type          string        `json:"type"`
	Source        string        `json:"source,omitempty"`
	Score         float64       `json:"score"`
	MatchedFields []SearchField `json:"matched_fields"`
	Snippet       *string       `json:"snippet,omitempty"`
	RerankScore   *float64      `json:"rerank_score,omitempty"`
	ScorerScore   *float64      `json:"scorer_score,omitempty"`
}

// HybridHit is a combined search result.
type HybridHit struct {
	ID              string        `json:"id"`
	Title           string        `json:"title"`
	Path            string        `json:"path"`
	Type            string        `json:"type"`
	Source          string        `json:"source,omitempty"`
	KeywordScore    float64       `json:"keyword_score"`
	LexicalScore    float64       `json:"lexical_score,omitempty"`
	FuzzyScore      float64       `json:"fuzzy_score"`
	TotalScore      float64       `json:"total_score"`
	MatchedKeywords []string      `json:"matched_keywords"`
	MatchedFields   []SearchField `json:"matched_fields"`
	Snippet         *string       `json:"snippet,omitempty"`
	RerankScore     *float64      `json:"rerank_score,omitempty"`
	ScorerScore     *float64      `json:"scorer_score,omitempty"`
}

// LexicalHit is one BM25 lexical search result.
type LexicalHit struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Path        string   `json:"path"`
	Type        string   `json:"type"`
	Source      string   `json:"source,omitempty"`
	RawScore    float64  `json:"raw_score"`
	Score       float64  `json:"score"`
	RerankScore *float64 `json:"rerank_score,omitempty"`
	ScorerScore *float64 `json:"scorer_score,omitempty"`
}

// SemanticHit is one semantic search result.
type SemanticHit struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Path           string   `json:"path"`
	Type           string   `json:"type"`
	Source         string   `json:"source,omitempty"`
	Keywords       []string `json:"keywords,omitempty"`
	MediaType      string   `json:"media_type,omitempty"`
	SemanticScore  float64  `json:"semantic_score"`
	EmbeddingModel string   `json:"embedding_model"`
	EmbeddingDim   int      `json:"embedding_dim"`
}

// ScorerInfo describes the scorer applied to one search response.
type ScorerInfo struct {
	Name       string  `json:"name"`
	Weight     float64 `json:"weight"`
	CandidateK int     `json:"candidate_k"`
}

// SearchResponse contains the keyword, fuzzy, hybrid, lexical and semantic top-K lists.
type SearchResponse struct {
	Query    string        `json:"query"`
	Mode     SearchMode    `json:"mode"`
	TopK     int           `json:"top_k"`
	Keyword  []KeywordHit  `json:"keyword"`
	Fuzzy    []FuzzyHit    `json:"fuzzy"`
	Hybrid   []HybridHit   `json:"hybrid"`
	Lexical  []LexicalHit  `json:"lexical,omitempty"`
	Semantic []SemanticHit `json:"semantic,omitempty"`
	Scorer   *ScorerInfo   `json:"scorer,omitempty"`
}

// ListResult is one metadata-filtered concept.
type ListResult struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Path        string         `json:"path"`
	Type        string         `json:"type"`
	Source      string         `json:"source,omitempty"`
	Keywords    []string       `json:"keywords"`
	Status      *ConceptStatus `json:"status,omitempty"`
	Tags        []string       `json:"tags,omitempty"`
	ContentKind string         `json:"content_kind,omitempty"`
	MediaType   string         `json:"media_type,omitempty"`
}

type ListResults []ListResult

// ShowLink is an outbound link on a concept page.
type ShowLink struct {
	Raw              string `json:"raw"`
	Target           string `json:"target"`
	ResolvesInBundle bool   `json:"resolves_in_bundle"`
}

// ShowResult is the detailed concept view.
type ShowResult struct {
	ID              string             `json:"id"`
	Path            string             `json:"path"`
	Type            string             `json:"type"`
	Source          string             `json:"source,omitempty"`
	Keywords        []string           `json:"keywords"`
	Title           *string            `json:"title,omitempty"`
	Description     *string            `json:"description,omitempty"`
	Tags            []string           `json:"tags,omitempty"`
	Status          *ConceptStatus     `json:"status,omitempty"`
	Body            string             `json:"body"`
	Links           []ShowLink         `json:"links"`
	Backlinks       []string           `json:"backlinks"`
	Privacy         *PrivacyState      `json:"privacy,omitempty"`
	ExtensionData   []ExtensionDataRef `json:"extension_data,omitempty"`
	ContentKind     string             `json:"content_kind,omitempty"`
	MediaType       string             `json:"media_type,omitempty"`
	ContentMetadata map[string]any     `json:"content_metadata,omitempty"`
	BodyForm        string             `json:"body_form,omitempty"`
	BodyCodec       string             `json:"body_codec,omitempty"`
	Size            int64              `json:"size,omitempty"`
	StoredSize      int64              `json:"stored_size,omitempty"`
	ContentHash     string             `json:"content_hash,omitempty"`
	BlobURI         string             `json:"blob_uri,omitempty"`
	BodyBytes       []byte             `json:"-"`
}

// ExtensionDataRef is one opaque extension-owned artifact bound to a document.
type ExtensionDataRef struct {
	Extension string         `json:"extension"`
	Key       string         `json:"key"`
	MediaType string         `json:"media_type,omitempty"`
	Size      int64          `json:"size"`
	SHA256    string         `json:"sha256"`
	Metadata  map[string]any `json:"metadata,omitempty"`
}

// Backlink is a concept that references another concept.
type Backlink struct {
	From  string `json:"from"`
	Title string `json:"title"`
}

// BacklinksResult lists incoming references.
type BacklinksResult struct {
	ID        string     `json:"id"`
	Backlinks []Backlink `json:"backlinks"`
}

// PathResult is a shortest directed path, or null when disconnected.
type PathResult struct {
	From   string   `json:"from"`
	To     string   `json:"to"`
	Source string   `json:"source,omitempty"`
	Path   []string `json:"path"`
}

// AuditFinding is one conformance or graph-quality issue.
type AuditFinding struct {
	Code     AuditFindingCode `json:"code"`
	Severity AuditSeverity    `json:"severity"`
	Source   string           `json:"source,omitempty"`
	Concept  string           `json:"concept,omitempty"`
	Target   string           `json:"target,omitempty"`
	Message  string           `json:"message"`
}

// AuditSummary contains deterministic counts for an audit report.
type AuditSummary struct {
	Concepts            int  `json:"concepts"`
	Valid               bool `json:"valid"`
	Edges               int  `json:"edges"`
	Orphans             int  `json:"orphans"`
	BrokenLinks         int  `json:"broken_links"`
	MissingKeywords     int  `json:"missing_keywords"`
	MissingIndexTargets int  `json:"missing_index_targets"`
	UnindexedConcepts   int  `json:"unindexed_concepts"`
}

// AuditReport is the result of `weave audit`.
type AuditReport struct {
	Path     string         `json:"path"`
	OK       bool           `json:"ok"`
	FailOn   AuditFailOn    `json:"fail_on"`
	Summary  AuditSummary   `json:"summary"`
	Findings []AuditFinding `json:"findings"`
}

// RelateEdge is a traversed graph edge annotated with its hop depth.
type RelateEdge struct {
	From   string  `json:"from"`
	To     string  `json:"to"`
	Source string  `json:"source"`
	Raw    *string `json:"raw,omitempty"`
	Depth  int     `json:"depth"`
}

// RelateResult is the bounded graph neighborhood returned by `weave relate`.
type RelateResult struct {
	Root      string          `json:"root"`
	Source    string          `json:"source,omitempty"`
	Direction RelateDirection `json:"direction"`
	Depth     int             `json:"depth"`
	Nodes     []GraphNode     `json:"nodes"`
	Edges     []RelateEdge    `json:"edges"`
}

type ExplainKeywordMatch string

const (
	ExplainMatchExact  ExplainKeywordMatch = "exact"
	ExplainMatchPrefix ExplainKeywordMatch = "prefix"
	ExplainMatchNone   ExplainKeywordMatch = "none"
)

// ExplainKeywordToken is per-query-token keyword evidence.
type ExplainKeywordToken struct {
	QueryToken     string              `json:"query_token"`
	MatchedKeyword string              `json:"matched_keyword"`
	Match          ExplainKeywordMatch `json:"match"`
	Score          float64             `json:"score"`
}

// ExplainKeyword is the keyword component of an explanation.
type ExplainKeyword struct {
	Score           float64               `json:"score"`
	MatchedKeywords []string              `json:"matched_keywords"`
	Tokens          []ExplainKeywordToken `json:"tokens"`
}

// ExplainFuzzyField is one field's fuzzy score and weighted contribution.
type ExplainFuzzyField struct {
	Field        SearchField `json:"field"`
	Score        float64     `json:"score"`
	Weight       float64     `json:"weight"`
	Contribution float64     `json:"contribution"`
}

// ExplainFuzzyToken is per-query-token fuzzy evidence.
type ExplainFuzzyToken struct {
	QueryToken   string      `json:"query_token"`
	MatchedToken string      `json:"matched_token"`
	Field        SearchField `json:"field"`
	Similarity   float64     `json:"similarity"`
}

// ExplainFuzzy is the fuzzy component of an explanation.
type ExplainFuzzy struct {
	Score         float64             `json:"score"`
	MatchedFields []SearchField       `json:"matched_fields"`
	Fields        []ExplainFuzzyField `json:"fields"`
	Tokens        []ExplainFuzzyToken `json:"tokens"`
	Snippet       *string             `json:"snippet,omitempty"`
}

// ExplainHybrid is the hybrid component of an explanation.
type ExplainHybrid struct {
	KeywordWeight float64 `json:"keyword_weight"`
	LexicalWeight float64 `json:"lexical_weight,omitempty"`
	FuzzyWeight   float64 `json:"fuzzy_weight"`
	Total         float64 `json:"total"`
}

// ExplainLexicalToken is per-query-token BM25 evidence.
type ExplainLexicalToken struct {
	QueryToken   string  `json:"query_token"`
	Matched      bool    `json:"matched"`
	Contribution float64 `json:"contribution"`
}

// ExplainLexical is the BM25 component of an explanation.
type ExplainLexical struct {
	RawScore      float64               `json:"raw_score"`
	Score         float64               `json:"score"`
	MatchedFields []SearchField         `json:"matched_fields"`
	Tokens        []ExplainLexicalToken `json:"tokens"`
}

// ExplainResult is the result of `weave explain`.
type ExplainResult struct {
	Query   string          `json:"query"`
	Mode    SearchMode      `json:"mode"`
	TopK    int             `json:"top_k"`
	ID      string          `json:"id"`
	Source  string          `json:"source,omitempty"`
	Rank    int             `json:"rank"`
	Found   bool            `json:"found"`
	Keyword ExplainKeyword  `json:"keyword"`
	Fuzzy   ExplainFuzzy    `json:"fuzzy"`
	Hybrid  ExplainHybrid   `json:"hybrid"`
	Lexical *ExplainLexical `json:"lexical,omitempty"`
	Scorer  *ExplainScorer  `json:"explain_scorer,omitempty"`
}

// ExplainScorer is the scorer evidence for one explained result.
type ExplainScorer struct {
	Name        string  `json:"name"`
	Weight      float64 `json:"weight"`
	CandidateK  int     `json:"candidate_k"`
	CoreScore   float64 `json:"core_score"`
	ScorerScore float64 `json:"scorer_score"`
	FinalScore  float64 `json:"final_score"`
}

// IngestReport is the result of `weave ingest` or `weave rebuild`.
type IngestReport struct {
	DB        string         `json:"db"`
	Bundle    string         `json:"bundle"`
	Mode      string         `json:"mode"`
	Documents int            `json:"documents"`
	Edges     int            `json:"edges"`
	Synthetic int            `json:"synthetic"`
	Warnings  int            `json:"warnings"`
	Errors    int            `json:"errors"`
	DryRun    bool           `json:"dry_run,omitempty"`
	Sources   []SourceReport `json:"sources,omitempty"`
}

// SyncReport is the result of `weave sync`.
type SyncReport struct {
	DB        string         `json:"db"`
	Bundle    string         `json:"bundle"`
	Mode      string         `json:"mode"`
	Documents int            `json:"documents"`
	Added     int            `json:"added"`
	Updated   int            `json:"updated"`
	Removed   int            `json:"removed"`
	Skipped   int            `json:"skipped"`
	Warnings  int            `json:"warnings"`
	Errors    int            `json:"errors"`
	DryRun    bool           `json:"dry_run,omitempty"`
	Sources   []SourceReport `json:"sources,omitempty"`
}

// OperationDescriptor describes one machine operation for discovery.
type OperationDescriptor struct {
	Op     OperationName `json:"op"`
	Fields []string      `json:"fields"`
	Result string        `json:"result"`
}

// ExtensionInfo is one discovered weave-<name> binary.
type ExtensionInfo struct {
	Name   string `json:"name"`
	Path   string `json:"path"`
	Source string `json:"source"`
}

// ExtensionList is the result of `weave extensions list`.
type ExtensionList struct {
	Extensions  []ExtensionInfo `json:"extensions"`
	SearchPaths []string        `json:"search_paths,omitempty"`
}

// ContractInfo is returned by `weave contract --json`.
type ContractInfo struct {
	Contract      string                `json:"contract"`
	CLI           string                `json:"cli"`
	BinaryVersion string                `json:"binary_version"`
	Operations    []OperationDescriptor `json:"operations"`
	Errors        []ErrorCode           `json:"errors"`
}

// VersionInfo is returned by `weave version --json`.
type VersionInfo struct {
	BinaryVersion string `json:"binary_version"`
	Contract      string `json:"contract"`
	CLI           string `json:"cli"`
}

// ConfigEntry is one resolved configuration key with provenance.
type ConfigEntry struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Source string `json:"source"`
	Origin string `json:"origin,omitempty"`
}

// ConfigShow is the typed result of `weave config show --json`.
type ConfigShow struct {
	ConfigPath string        `json:"config_path"`
	Exists     bool          `json:"exists"`
	Entries    []ConfigEntry `json:"entries"`
}

// ConfigGet is the typed result of `weave config get <key> --json`.
type ConfigGet struct {
	Key    string `json:"key"`
	Value  any    `json:"value"`
	Source string `json:"source"`
	Origin string `json:"origin,omitempty"`
}

// ConfigPath is the typed result of `weave config path --json`.
type ConfigPath struct {
	Path   string `json:"path"`
	Exists bool   `json:"exists"`
}

// ConfigInit is the typed result of `weave config init --json`.
type ConfigInit struct {
	Path        string `json:"path"`
	Created     bool   `json:"created"`
	NextCommand string `json:"next_command,omitempty"`
}

// ConfigPaths is the typed result of `weave config paths --json`.
type ConfigPaths struct {
	Config     string   `json:"config"`
	Database   string   `json:"database"`
	Cache      string   `json:"cache"`
	Keys       []string `json:"keys"`
	Extensions []string `json:"extensions"`
}

// WatchEvent is one JSONL event emitted by `weave sync --watch --events --json`.
type WatchEvent struct {
	Event   string `json:"event"`
	At      string `json:"at"`
	Added   int    `json:"added"`
	Updated int    `json:"updated"`
	Removed int    `json:"removed"`
	Errors  int    `json:"errors"`
}

// SourceSpec is one bundle source passed to a multi-source invocation.
type SourceSpec struct {
	Alias      string `json:"alias"`
	Path       string `json:"path"`
	Provider   string `json:"provider,omitempty"`
	Ref        string `json:"ref,omitempty"`
	Worktree   bool   `json:"worktree,omitempty"`
	Submodules bool   `json:"submodules,omitempty"`
	LFS        bool   `json:"lfs,omitempty"`
}

// SourceContext carries materialized source provenance through an in-process
// write. It is never serialized on the wire.
type SourceContext struct {
	Alias           string
	Provider        string
	URI             string
	Ref             string
	Commit          string
	SnapshotPath    string
	RepositoryName  string
	RepositoryTitle string
	Description     string
	Readme          string
	Host            string
	Owner           string
	License         string
	Worktree        bool
	Warnings        []string
	Projects        []SourceProject
}

// SourceProject is one manifest-backed project discovered in a source snapshot.
type SourceProject struct {
	Path         string
	Name         string
	Description  string
	Manifest     string
	Ecosystem    string
	Dependencies []SourceDependency
}

// SourceDependency is one declared project dependency.
type SourceDependency struct {
	Name    string
	Version string
	Scope   string
}

// SourceBundle is one per-source parse result.
type SourceBundle struct {
	Source string  `json:"source"`
	Bundle *Bundle `json:"bundle"`
}

// SourceBundles is the result of a multi-source parse.
type SourceBundles struct {
	Sources []SourceBundle `json:"sources"`
}

// SourceGraph is one per-source graph result.
type SourceGraph struct {
	Source string `json:"source"`
	Graph  Graph  `json:"graph"`
}

// SourceGraphs is the result of a multi-source graph.
type SourceGraphs struct {
	Sources []SourceGraph `json:"sources"`
}

// SourceReport is one per-source write summary.
type SourceReport struct {
	Source    string `json:"source"`
	Documents int    `json:"documents"`
	Added     int    `json:"added,omitempty"`
	Updated   int    `json:"updated,omitempty"`
	Removed   int    `json:"removed,omitempty"`
	Skipped   int    `json:"skipped,omitempty"`
	Warnings  int    `json:"warnings"`
	Errors    int    `json:"errors"`
}

// ExportReport is the result of `weave export`.
type ExportReport struct {
	Source               string `json:"source"`
	Out                  string `json:"out"`
	Format               string `json:"format"`
	Documents            int    `json:"documents"`
	Blobs                int    `json:"blobs,omitempty"`
	Encrypted            int    `json:"encrypted"`
	Tokenized            int    `json:"tokenized"`
	Hashed               int    `json:"hashed"`
	ExtensionData        int    `json:"extension_data,omitempty"`
	ExtensionDataSkipped int    `json:"extension_data_skipped,omitempty"`
	DryRun               bool   `json:"dry_run,omitempty"`
	Confirmation         bool   `json:"confirmation_required,omitempty"`
}

// CodecInfo describes one available body codec.
type CodecInfo struct {
	Name          string `json:"name"`
	Source        string `json:"source"`
	Path          string `json:"path,omitempty"`
	CanCompress   bool   `json:"can_compress"`
	CanDecompress bool   `json:"can_decompress"`
}

// CodecList is the result of `weave codecs list`.
type CodecList struct {
	Codecs []CodecInfo `json:"codecs"`
}

// OperationRequest is the transport-neutral request shape for stdio/MCP.
type OperationRequest struct {
	ID            string          `json:"id,omitempty"`
	Op            OperationName   `json:"op"`
	Path          string          `json:"path,omitempty"`
	Query         string          `json:"query,omitempty"`
	Mode          SearchMode      `json:"mode,omitempty"`
	TopK          *int            `json:"top_k,omitempty"`
	Target        string          `json:"target,omitempty"`
	From          string          `json:"from,omitempty"`
	To            string          `json:"to,omitempty"`
	Types         []string        `json:"types,omitempty"`
	Tags          []string        `json:"tags,omitempty"`
	Status        string          `json:"status,omitempty"`
	Limit         *int            `json:"limit,omitempty"`
	FailOn        AuditFailOn     `json:"fail_on,omitempty"`
	VerifyKeys    bool            `json:"verify_keys,omitempty"`
	Decrypt       bool            `json:"decrypt,omitempty"`
	Depth         *int            `json:"depth,omitempty"`
	Direction     RelateDirection `json:"direction,omitempty"`
	Relations     []RelationKind  `json:"relations,omitempty"`
	DB            string          `json:"db,omitempty"`
	NoDB          bool            `json:"no_db,omitempty"`
	Strict        bool            `json:"strict,omitempty"`
	Permissive    bool            `json:"permissive,omitempty"`
	Assets        bool            `json:"assets,omitempty"`
	NoAssets      bool            `json:"no_assets,omitempty"`
	AssetTypes    []string        `json:"asset_types,omitempty"`
	Codec         string          `json:"codec,omitempty"`
	CodecSet      bool            `json:"codec_set,omitempty"`
	Enricher      string          `json:"enricher,omitempty"`
	Embedder      string          `json:"embedder,omitempty"`
	Rerank        string          `json:"rerank,omitempty"`
	Scorer        string          `json:"scorer,omitempty"`
	ScorerWeight  *float64        `json:"scorer_weight,omitempty"`
	CandidateK    *int            `json:"candidate_k,omitempty"`
	TimeoutMS     int64           `json:"timeout_ms,omitempty"`
	Sources       []SourceSpec    `json:"sources,omitempty"`
	Source        *SourceContext  `json:"-"`
	WorkspaceRoot string          `json:"-"`
	Out           string          `json:"out,omitempty"`
	Format        string          `json:"format,omitempty"`
	KeyID         string          `json:"key_id,omitempty"`
	Provider      string          `json:"provider,omitempty"`
	Force         bool            `json:"force,omitempty"`
	Confirmed     bool            `json:"confirmed,omitempty"`
	DryRun        bool            `json:"dry_run,omitempty"`
}

// OperationError is the stable error shape shared by CLI and future transports.
type OperationError struct {
	Code    ErrorCode      `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// SuccessEnvelope is a successful machine response.
type SuccessEnvelope struct {
	ID       string `json:"id,omitempty"`
	Contract string `json:"contract"`
	CLI      string `json:"cli"`
	OK       bool   `json:"ok"`
	Result   any    `json:"result"`
}

// FailureEnvelope is a failed machine response.
type FailureEnvelope struct {
	ID       string         `json:"id,omitempty"`
	Contract string         `json:"contract"`
	CLI      string         `json:"cli"`
	OK       bool           `json:"ok"`
	Error    OperationError `json:"error"`
}
