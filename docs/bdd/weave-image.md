# BDD: Weave Image Extension

Human-readable behavior cases for `weave-image` as the first generic blob consumer.

**Spec reference:** [weave-image.md](https://github.com/prulloac/weave-docs/blob/main/specs/weave-image.md)
**Go tests:** planned: `cmd/weave/weave_image_bdd_test.go`, `cmd/weave/blob_assets_bdd_test.go`

`weave-image` enriches image blobs with OCR, EXIF, dimensions and `weave-ai` vision metadata. Core
stays media-agnostic; this document covers the integration behavior in `weave`.

---

## Feature: Capability and Dependency

### Scenario: Capability manifest advertises image support

- **When** I run `weave-image --describe --json`
- **Then** it returns `weave.extension/v1`
- **And** `asset_types` contains JPEG, PNG, WebP and GIF
- **And** `requires` names `weave >= 1.0.0` and `weave-ai >= 1.0.0`

### Scenario: Missing weave-ai still ingests the image

- **Given** `weave-image` is installed but `weave-ai` is not on `PATH`
- **When** I ingest an image with `--assets --with-enricher image`
- **Then** the image document is stored
- **And** local OCR, EXIF and dimensions are kept
- **And** the report counts a warning
- **And** the warning names the missing `weave-ai` dependency

### Scenario: Old weave-ai is rejected

- **Given** `weave-ai` reports a version below `1.0.0`
- **When** `weave-image` tries vision enrichment
- **Then** the image document is stored with local metadata only
- **And** the warning names the required version

---

## Feature: Image Discovery and Validation

### Scenario: Supported formats are ingested

- **Given** JPEG, PNG, WebP and GIF files in the bundle
- **When** I ingest with `--assets --with-enricher image`
- **Then** each file becomes a blob document
- **And** each `media_type` matches its format

### Scenario: Unsupported files are skipped

- **Given** a text file in a bundle scanned for images
- **When** I ingest with `--assets --with-enricher image`
- **Then** the text file is not passed to `weave-image`
- **And** no image document is created for it

### Scenario: MIME and extension mismatch is a warning

- **Given** a `.jpg` file whose magic bytes identify PNG
- **When** I ingest it with `--assets --with-enricher image`
- **Then** the detected media type is used
- **And** the report counts one warning

### Scenario: Oversized image is skipped

- **Given** an image above the 32 MiB blob limit
- **When** I ingest with `--assets --with-enricher image`
- **Then** the image is skipped with a warning
- **And** `--strict` fails

---

## Feature: OCR and EXIF

### Scenario: Local OCR text is stored and searchable

- **Given** a local OCR backend that returns `Opening hours`
- **When** I ingest the image with `--assets --with-enricher image`
- **Then** `content_metadata.text` contains the OCR text
- **And** `content_metadata.text_source` is `ocr`
- **And** `weave search "Opening hours"` finds the image

### Scenario: Local OCR wins over vision transcription

- **Given** local OCR returns text and `weave-ai` returns different text
- **When** I ingest the image
- **Then** the local OCR text is stored
- **And** the vision text does not replace it

### Scenario: Vision transcription is used without local OCR

- **Given** no local OCR backend and a `weave-ai` response containing text
- **When** I ingest the image
- **Then** `content_metadata.text` contains the vision text
- **And** `content_metadata.text_source` is `vision`

### Scenario: EXIF is stored and sent to AI

- **Given** a JPEG with camera and GPS EXIF
- **When** I ingest the image
- **Then** `content_metadata.attributes.exif` contains the EXIF object
- **And** the fake `weave-ai` receives the same EXIF in its request

### Scenario: Dimensions are stored

- **Given** a 1280×960 image
- **When** I ingest the image
- **Then** `content_metadata.attributes.width` is 1280
- **And** `height` is 960

---

## Feature: AI Metadata

### Scenario: AI fills empty title and description

- **Given** an image document with empty title and description
- **When** `weave-ai` returns a title and description
- **Then** both fields are stored
- **And** `weave search` can match the new terms

### Scenario: Declared metadata wins

- **Given** an image document with a declared title and description
- **When** `weave-ai` returns different values
- **Then** the declared values are preserved unchanged

### Scenario: AI keywords append

- **Given** an image with declared keywords and AI keywords
- **When** I ingest the image
- **Then** AI keywords are appended
- **And** duplicates are removed case-insensitively

### Scenario: AI failure keeps local metadata

- **Given** `weave-ai` times out or returns malformed JSON
- **When** I ingest the image
- **Then** the image document is stored
- **And** OCR, EXIF and dimensions from local processing remain
- **And** the report counts a warning

### Scenario: No OCR and no AI text remains metadata-only

- **Given** no OCR backend and an AI response without text
- **When** I ingest the image
- **Then** the image is stored
- **And** it is searchable by title, description or keywords only

---

## Feature: Retrieval

### Scenario: Image bytes round-trip

- **Given** an indexed image document
- **When** I run `weave show <path> <id> --out image.png`
- **Then** the output is byte-for-byte identical to the source image

### Scenario: JSON returns image metadata only

- **When** I run `weave show <path> <id> --json`
- **Then** it includes media type, EXIF, dimensions and OCR text
- **And** it does not embed image bytes

### Scenario: MCP serves the image as a blob

- **Given** an MCP server over the bundle
- **When** I read `weave://blob/<id>`
- **Then** the resource content is a blob
- **And** the bytes match the source image
- **And** the MIME type is the image media type

---

## Feature: Core Boundary

### Scenario: Core does not special-case images

- **Given** an image, a PDF and an arbitrary binary fixture
- **When** I ingest them with matching asset types and a fake enricher
- **Then** each follows the same blob document lifecycle
- **And** core does not call OCR, EXIF or vision code directly
