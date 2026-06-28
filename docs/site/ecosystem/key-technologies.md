# Key Technologies

A structured map of the core technologies behind RAG systems and LLM Wiki systems — two distinct paradigms for building AI knowledge infrastructure.

## The Two Paradigms

```
RAG Pipeline                        LLM Wiki (Knowledge Compilation)
─────────────────────────────       ─────────────────────────────────
Query time: retrieve → generate     Write time: compile → index → read

Raw docs → chunk → embed → store    Raw docs → LLM distills → wiki pages
Query → vector search → LLM         Query → FTS / graph → read compiled page
```

> **Key difference:** RAG retrieves raw fragments at query time. LLM Wiki pre-compiles knowledge into structured, auditable pages — retrieval is cheap, knowledge accumulates over time.

---

## RAG System Technologies

### 1. Document Parsing

| Technology | Representative Tools | Notes |
|---|---|---|
| Multi-format parsing | [Unstructured.io](https://unstructured.io), [Docling](https://github.com/DS4SD/docling) (IBM) | Layout-aware extraction from PDF/Word/HTML |
| PDF parsing | PyMuPDF, PDFPlumber | Low-level text/table/layout extraction |
| LLM-enhanced parsing | LlamaParse | Cloud API for complex PDFs with tables and figures |
| PDF → Markdown | [Marker](https://github.com/VikParuchuri/marker), [MinerU](https://github.com/opendatalab/MinerU) | High-fidelity conversion, preserves structure |
| OCR cloud services | Azure Document Intelligence, AWS Textract | Enterprise-grade form and table recognition |

### 2. Chunking Strategies

| Strategy | Tools | Notes |
|---|---|---|
| Fixed-length chunking | LangChain `CharacterTextSplitter` | Simple, breaks semantic boundaries |
| Recursive splitting | LangChain `RecursiveCharacterTextSplitter` | Respects paragraph → sentence → word hierarchy |
| Semantic chunking | LlamaIndex `SemanticSplitter` | Detects topic boundaries via embedding similarity |
| Chapter-tree indexing | [he-wiki-rag](https://github.com/liuhe37186/he-wiki-rag) | Preserves H1/H2/H3 hierarchy + breadcrumb path |
| Parent-child chunking | LlamaIndex | Retrieve small chunks, return parent as context |
| Proposition chunking | Custom + LLM | Split into atomic facts — highest precision, expensive |
| Late Chunking | Jina AI | Embed full document first, then split embeddings |
| RAPTOR | LlamaIndex | Recursive tree summaries for hierarchical retrieval |
| Document-tree reasoning | [PageIndex](https://github.com/vectifyai/vectify) | LLM traverses summary tree — no vectors needed, 98.7% on FinanceBench |

### 3. Embedding Models

| Model | Provider | Notes |
|---|---|---|
| text-embedding-3-large/small | OpenAI | General-purpose, 3072-dim large variant |
| Embed v3 | Cohere | Optimized for retrieval, 1024-dim |
| BGE-M3 | BAAI (open source) | Multilingual, multi-granularity, strong Chinese |
| E5-Mistral-7B | Microsoft (open source) | High-dim, top MTEB ranking |
| NV-Embed-v2 | NVIDIA (open source) | MTEB leader among open models |
| nomic-embed-text | Nomic (open source) | Fully open, local-friendly |
| jina-embeddings-v3 | Jina AI (open source) | Supports Late Chunking |

### 4. Vector Databases

| Database | Type | Notes |
|---|---|---|
| [Qdrant](https://github.com/qdrant/qdrant) | Open source | Rust, high-performance, strong filtering. 100k–10M scale. |
| [Milvus](https://github.com/milvus-io/milvus) | Open source | Billion-scale distributed. Commercial: Zilliz Cloud. |
| [Weaviate](https://github.com/weaviate/weaviate) | Open source | Native hybrid search (vector + BM25) |
| [Chroma](https://github.com/chroma-core/chroma) | Open source | Lightweight, embedded, best for prototyping |
| [pgvector](https://github.com/pgvector/pgvector) | Extension | Vector search inside PostgreSQL |
| [LanceDB](https://github.com/lancedb/lancedb) | Open source | Arrow format, embedded, serverless-friendly |
| Pinecone | Managed SaaS | Serverless, zero ops |

### 5. Retrieval Strategies

| Strategy | Tools | Notes |
|---|---|---|
| Dense retrieval | All vector DBs (ANN) | Cosine/dot-product semantic similarity |
| Sparse retrieval (BM25) | Elasticsearch, OpenSearch, Tantivy | Term-frequency keyword matching |
| Hybrid retrieval | Weaviate, Qdrant, RRF algorithm | Dense + sparse, merged via Reciprocal Rank Fusion |
| Graph-augmented retrieval | [GraphRAG](https://github.com/microsoft/graphrag), [LightRAG](https://github.com/HKUDS/LightRAG) | Entity/relation graph for multi-hop reasoning |
| Vector Graph RAG | Community | Triples vectorized instead of graph DB — 96.3% on HotpotQA |
| Agentic RAG (A-RAG) | Custom + LLM | Agent autonomously chooses `keyword_search` / `semantic_search` / `chunk_read` tools |

### 6. Query Optimization

| Technique | Tools | Notes |
|---|---|---|
| HyDE | LangChain, LlamaIndex | Generate hypothetical answer, use its embedding to retrieve |
| Multi-query / Query rewriting | LangChain `MultiQueryRetriever` | LLM rewrites to multiple sub-questions |
| Step-Back Prompting | LangChain | Abstract specific question to general before retrieval |
| Self-RAG | Research implementation | LLM self-evaluates retrieval quality, decides whether to re-retrieve |
| CRAG | Research implementation | Falls back to web search for low-confidence retrievals |

### 7. Reranking

| Tool | Notes |
|---|---|
| Cohere Rerank | Cross-encoder commercial reranker |
| BGE Reranker (BAAI) | Open-source cross-encoder, strong multilingual |
| FlashRank | Lightweight open-source reranker for local deployment |

### 8. Orchestration Frameworks

| Framework | Notes |
|---|---|
| [LangChain](https://github.com/langchain-ai/langchain) | Most popular modular RAG pipeline framework |
| [LlamaIndex](https://github.com/run-llama/llama_index) | Data-centric RAG, richest chunking/retrieval strategies |
| [Haystack](https://github.com/deepset-ai/haystack) | Enterprise-grade NLP/RAG pipeline |
| [DSPy](https://github.com/stanfordnlp/dspy) | Programmatic LLM optimization, replaces hand-written prompts |

### 9. Evaluation

| Tool | Notes |
|---|---|
| [RAGAS](https://ragas.io) | Most popular RAG eval — faithfulness, context recall, answer relevancy |
| [DeepEval](https://github.com/confident-ai/deepeval) | Unit-test style RAG evaluation |
| [TruLens](https://www.trulens.org) | Production monitoring via RAG Triad |
| LangSmith | LangChain's tracing and eval platform |
| Key metrics | Faithfulness, Context Recall, Answer Relevancy, MRR, NDCG |

---

## LLM Wiki Technologies

### 1. Knowledge Compilation

| Technology | Notes |
|---|---|
| LLM as compiler | LLM reads raw docs and writes structured wiki pages — pre-compiled, not retrieved at query time. Coined by Andrej Karpathy. |
| Tiered compilation | Compile frequently-used docs first (Tier 0–3). Sage Wiki: auto-promote on 3 hits, auto-demote after 90 days inactive. |
| Incremental update | Only re-process changed documents — avoids full rebuild cost. |
| Distillation pipeline | Raw doc → LLM extracts key claims, entities, aliases, related links → source-note page (WikiLoop pattern). |
| [STORM](https://github.com/stanford-oval/storm) (Stanford OVAL) | Multi-perspective Wiki generation agent: simulates different viewpoints → retrieves → generates hierarchical outline → writes cited article. |
| Co-STORM | Collaborative variant of STORM — builds dynamic knowledge map during research to guide compilation direction. |
| Agentic RAG compilation | Agent loop: retrieve → draft → evaluate → re-retrieve → regenerate until knowledge is stable. |

### 2. Knowledge Representation

| Technology | Notes |
|---|---|
| Structured Markdown | All knowledge stored as plain Markdown — human-readable, git-diffable, auditable. |
| Source-note pages | One distilled page per raw document. Contains `key_claims`, entity annotations `【entity\|type】`, `related_to` / `supports` / `contradicts` links. |
| Concept / Comparison / Decision pages | Cross-document synthesis: concept explanations, side-by-side comparisons, technical decision records. |
| Ontology graph | Typed entity-relation graph built during compilation. Sage Wiki: 8 built-in relation types (`implements`, `contradicts`, `trades_off`, …). |
| Schema / Templates | Authoring rules and page templates that guide LLM compilation style, customizable per KB. |

### 3. Indexing & Search

| Technology | Notes |
|---|---|
| SQLite FTS5 + BM25 | Core search engine — no vector model needed. Sub-millisecond full-text search. Used by WikiLoop, Sage Wiki, TreeSearch. |
| Alias expansion | Key terms indexed with aliases and cross-language equivalents to maximize FTS recall. |
| Graph traversal | `related_to` links enable multi-hop navigation (similar to wiki page links). BFS expansion on search results. |
| Hybrid (FTS + vector + graph) | Sage Wiki: FTS5 (411µs) + vector (81ms) + ontology graph (1µs) merged via RRF. |
| Chapter-tree indexing | Preserves document H1/H2/H3 hierarchy — structure-preserving alternative to flat chunking. |

### 4. Knowledge Quality & Maintenance

| Technology | Notes |
|---|---|
| Git version control | All wiki pages in git — full history, diffs, blame. Knowledge changes are auditable. |
| Lint / health checks | Validate frontmatter, broken source links, missing citations. `wikiloop lint`. |
| Conflict detection | `contradicts` links surface disagreements between sources for human review. |
| Draft staging | Pages with < 2 source references quarantined in `_draft/` — not indexed until verified. |
| Knowledge gap analysis | `wikiloop synthesize --gaps` identifies topics with insufficient coverage. |

### 5. Agent Interface (MCP)

| Technology | Notes |
|---|---|
| [MCP protocol](https://modelcontextprotocol.io) | Model Context Protocol — Anthropic open standard for exposing tools/resources to AI agents. Supports stdio + HTTP transports. |
| `kb_search` | FTS keyword search, returns ranked results with `related` links for graph navigation. |
| `kb_page` | Fetch full page content by ID. Supports batch (up to 5 IDs) or `full=true` for untruncated text. |
| MCP Resources | Read-only resources (URI form): wiki pages, graph schema, raw documents. |
| Iterative search pattern | Agent issues multiple queries from different angles, follows `related` links, synthesizes own answer. |
| Sage Wiki MCP tools | 17 tools: 6 read, 9 write, 2 composite — agents can directly write and compile knowledge. |

### 6. File Conversion (Input Layer)

| Technology | Notes |
|---|---|
| markitdown (Microsoft) | Converts PDF, Word, Excel, PPT, HTML to Markdown before distillation. |
| `raw/converted/` pattern | Agent-extracted content written directly here — skips conversion, goes straight to distillation. |
| File watcher | Auto-detects new/changed files in `raw/`, triggers convert → distill → index pipeline. |

---

## Technology Comparison

| Dimension | RAG | LLM Wiki |
|---|---|---|
| Core operation | Retrieve at query time | Compile at write time |
| Storage | Vector DB + raw chunks | Structured Markdown + SQLite FTS |
| Search | ANN similarity search | FTS5 BM25 + graph traversal |
| Knowledge form | Implicit (vectors) | Explicit (readable Markdown) |
| Auditability | Low | High (git diff, lint) |
| Multi-hop reasoning | LLM-dependent | Via `related` graph links |
| Embedding required | Yes | No (pure FTS) |
| Knowledge accumulation | None (static index) | Compounds over time |
| Token cost at query time | High (raw chunks) | Low (pre-compiled pages) |
| Best for | Broad doc Q&A, real-time data | Long-term knowledge, agent memory, research |
