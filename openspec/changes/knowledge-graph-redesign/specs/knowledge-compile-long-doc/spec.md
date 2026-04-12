## ADDED Requirements

### Requirement: Long document three-phase compilation (Scan → Gather → Write)
When a source's content exceeds the configured maxChunkSize (default: 40% of the LLM model's context window), the MAP phase SHALL use a three-phase strategy instead of processing the content in a single LLM call.

#### Scenario: Short source uses fast path
- **WHEN** a source's content length is ≤ maxChunkSize
- **THEN** system processes the entire content in a single LLM call (unchanged behavior)

#### Scenario: Long source triggers three-phase processing
- **WHEN** a source's content length exceeds maxChunkSize
- **THEN** system executes CHUNK → SCAN → MERGE → GATHER → WRITE phases for that source

#### Scenario: CHUNK phase splits by natural boundaries
- **WHEN** system chunks a long source
- **THEN** system splits at chapter/section headers first, then paragraph boundaries, then fixed-length as fallback
- **THEN** each chunk SHALL be ≤ maxChunkSize
- **THEN** each chunk SHALL retain its section/chapter title as metadata

#### Scenario: SCAN phase extracts lightweight concept list
- **WHEN** system processes each chunk in SCAN phase
- **THEN** LLM outputs only concept title + one-line summary per chunk (no full article content)
- **THEN** system records a concept → chunk_indices mapping for each extracted concept
- **THEN** SCAN calls for different chunks MAY execute in parallel

#### Scenario: MERGE phase deduplicates concepts
- **WHEN** system merges SCAN results from all chunks
- **THEN** system deduplicates concepts by exact title match
- **THEN** system produces a global concept list with each concept's associated chunk indices

#### Scenario: GATHER phase collects evidence from original text
- **WHEN** system gathers evidence for a concept
- **THEN** system extracts all paragraphs from the concept's associated chunks in the original source text
- **THEN** if the total evidence exceeds maxChunkSize, system ranks paragraphs by concept mention density and takes top-K paragraphs within the limit
- **THEN** truncated paragraphs SHALL retain their first 2 and last 2 sentences as snippets appended at the end

#### Scenario: WRITE phase produces full articles from evidence
- **WHEN** system writes an article for a concept
- **THEN** LLM receives the evidence bundle (original text, not summaries) and outputs a complete wiki article
- **THEN** WRITE calls for different concepts MAY execute in parallel
- **THEN** articles that fail the minContentLength check SHALL be discarded
