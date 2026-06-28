# 关键技术

RAG 系统和 LLM Wiki 系统背后核心技术的结构化全景图——两种构建 AI 知识基础设施的不同范式。

## 两种范式

```
RAG 管道                                LLM Wiki（知识编译范式）
─────────────────────────────────       ────────────────────────────────────
查询时：检索 → 生成                       写入时：编译 → 索引 → 读取

原始文档 → 切块 → 嵌入 → 存储            原始文档 → LLM 提炼 → Wiki 页面
查询 → 向量搜索 → LLM                     查询 → FTS / 图遍历 → 读取已编译页面
```

> **核心区别：** RAG 在查询时检索原始片段。LLM Wiki 在写入时将知识预编译为结构化、可审计的页面——检索成本低，知识随时间累积增厚。

---

## RAG 系统关键技术

### 1. 文档解析层

| 技术 | 代表工具 | 说明 |
|---|---|---|
| 多格式文档解析 | [Unstructured.io](https://unstructured.io)、[Docling](https://github.com/DS4SD/docling)（IBM） | 版面感知的 PDF/Word/HTML 多格式提取 |
| PDF 底层解析 | PyMuPDF、PDFPlumber | 文本/表格/布局结构提取 |
| LLM 增强解析 | LlamaParse | 处理含复杂表格和图形的 PDF 的云端 API |
| PDF 转 Markdown | [Marker](https://github.com/VikParuchuri/marker)、[MinerU](https://github.com/opendatalab/MinerU) | 高保真格式转换，保留文档结构 |
| OCR 云服务 | Azure Document Intelligence、AWS Textract | 企业级表单和表格识别 |

### 2. 文本切块策略

| 策略 | 工具 | 说明 |
|---|---|---|
| 固定长度切块 | LangChain `CharacterTextSplitter` | 简单，但会破坏语义边界 |
| 递归字符切割 | LangChain `RecursiveCharacterTextSplitter` | 按段落→句子→单词层级递归切割 |
| 语义切块 | LlamaIndex `SemanticSplitter` | 通过 Embedding 相似度检测主题边界 |
| 章节树索引 | [he-wiki-rag](https://github.com/liuhe37186/he-wiki-rag) | 保留 H1/H2/H3 层级和面包屑路径 |
| 父子分块 | LlamaIndex | 检索小粒度子块，返回父块作为上下文 |
| 命题切块 | 自定义 + LLM | 拆分为原子事实——精度最高，代价最大 |
| Late Chunking | Jina AI | 先嵌入完整文档再切分 Embedding |
| RAPTOR | LlamaIndex | 递归树状摘要，构建层次化检索结构 |
| 文档树推理 | [PageIndex](https://github.com/vectifyai/vectify) | LLM 遍历摘要树检索，无需向量，FinanceBench 98.7% |

### 3. Embedding 模型

| 模型 | 提供方 | 说明 |
|---|---|---|
| text-embedding-3-large/small | OpenAI | 通用嵌入，large 版 3072 维 |
| Embed v3 | Cohere | 专为检索优化，1024 维 |
| BGE-M3 | BAAI（开源） | 多语言多粒度，中文表现优异 |
| E5-Mistral-7B | Microsoft（开源） | 高维度，MTEB 排名靠前 |
| NV-Embed-v2 | NVIDIA（开源） | 开源模型中 MTEB 领先 |
| nomic-embed-text | Nomic（开源） | 完全开源，本地友好 |
| jina-embeddings-v3 | Jina AI（开源） | 支持 Late Chunking |

### 4. 向量数据库

| 数据库 | 类型 | 说明 |
|---|---|---|
| [Qdrant](https://github.com/qdrant/qdrant) | 开源 | Rust 实现，高性能，强过滤，适合 10 万~千万规模 |
| [Milvus](https://github.com/milvus-io/milvus) | 开源 | 十亿级分布式，商业版：Zilliz Cloud |
| [Weaviate](https://github.com/weaviate/weaviate) | 开源 | 原生混合检索（向量 + BM25） |
| [Chroma](https://github.com/chroma-core/chroma) | 开源 | 轻量嵌入式，原型开发首选 |
| [pgvector](https://github.com/pgvector/pgvector) | 扩展 | PostgreSQL 原生向量搜索扩展 |
| [LanceDB](https://github.com/lancedb/lancedb) | 开源 | Arrow 格式，嵌入式，无服务器友好 |
| Pinecone | 托管 SaaS | Serverless，零运维 |

### 5. 检索策略

| 策略 | 工具 | 说明 |
|---|---|---|
| 密集检索 | 各向量数据库 ANN | 余弦/点积语义相似度近似最近邻搜索 |
| 稀疏检索（BM25） | Elasticsearch、OpenSearch、Tantivy | 基于词频统计的关键词匹配 |
| 混合检索 | Weaviate、Qdrant、RRF 算法 | 密集 + 稀疏，通过 RRF 倒数排名融合 |
| 图增强检索 | [GraphRAG](https://github.com/microsoft/graphrag)、[LightRAG](https://github.com/HKUDS/LightRAG) | 实体/关系图谱支持多跳推理 |
| Vector Graph RAG | 社区 | 三元组向量化替代图数据库，HotpotQA 96.3% |
| Agentic RAG（A-RAG） | 自定义 + LLM | Agent 自主选择 `keyword_search` / `semantic_search` / `chunk_read` 工具 |

### 6. 查询优化

| 技术 | 工具 | 说明 |
|---|---|---|
| HyDE | LangChain、LlamaIndex | 生成假设性答案，用其 Embedding 做检索 |
| Multi-Query 查询改写 | LangChain `MultiQueryRetriever` | LLM 将原问题改写为多个子问题扩大召回 |
| Step-Back Prompting | LangChain | 将具体问题抽象为通用问题再检索 |
| Self-RAG | 论文实现 | LLM 自评估检索质量，决定是否继续检索 |
| CRAG | 论文实现 | 低置信度时回退到网络搜索 |

### 7. Rerank 精排

| 工具 | 说明 |
|---|---|
| Cohere Rerank | 交叉编码器商业精排模型 |
| BGE Reranker（BAAI） | 开源交叉编码器，多语言表现强 |
| FlashRank | 轻量开源精排库，适合本地部署 |

### 8. 编排框架

| 框架 | 说明 |
|---|---|
| [LangChain](https://github.com/langchain-ai/langchain) | 最流行的模块化 RAG 管道框架 |
| [LlamaIndex](https://github.com/run-llama/llama_index) | 数据中心型 RAG，最丰富的切块/检索策略 |
| [Haystack](https://github.com/deepset-ai/haystack) | 企业级 NLP/RAG 管道框架 |
| [DSPy](https://github.com/stanfordnlp/dspy) | 通过编程方式优化 LLM 管道，替代手写 Prompt |

### 9. 评估框架

| 工具 | 说明 |
|---|---|
| [RAGAS](https://ragas.io) | 最流行的 RAG 评估框架——忠实度、上下文召回、答案相关性 |
| [DeepEval](https://github.com/confident-ai/deepeval) | 单元测试风格的 RAG 评估库 |
| [TruLens](https://www.trulens.org) | 基于 RAG Triad 的生产监控工具 |
| LangSmith | LangChain 官方链路追踪与评估平台 |
| 核心指标 | Faithfulness、Context Recall、Answer Relevancy、MRR、NDCG |

---

## LLM Wiki 关键技术

### 1. 知识提炼 / 编译

| 技术 | 说明 |
|---|---|
| LLM 作为编译器 | LLM 读取原始文档，写入结构化 Wiki 页面——查询时直接读取编译产物，由 Karpathy 提出。 |
| 分层编译 | 优先编译高频文档（Tier 0–3）。Sage Wiki：命中 3 次自动升级，90 天不活跃自动降级。 |
| 增量更新 | 仅处理变更文档，避免全量重建成本。 |
| 提炼管道 | 原始文档 → LLM 提取关键主张、实体、别名、关联链接 → source-note 页面（WikiLoop 模式）。 |
| [STORM](https://github.com/stanford-oval/storm)（Stanford OVAL） | 多视角 Wiki 生成 Agent：模拟不同视角提问 → 检索 → 生成层级大纲 → 撰写带引用的完整词条。 |
| Co-STORM | STORM 的协作变体，研究过程中构建动态知识图谱指导编译方向。 |
| Agentic RAG 编译 | Agent 循环：检索 → 草稿 → 评估 → 补充检索 → 再生成，直到知识稳定。 |

### 2. 知识表示

| 技术 | 说明 |
|---|---|
| 结构化 Markdown | 所有知识以纯文本 Markdown 存储——人类可读，可 git diff，可审计。 |
| Source-note 页面 | 每个原始文档对应一个提炼笔记，包含 `key_claims`、实体标注 `【实体\|类型】`、`related_to` / `supports` / `contradicts` 链接。 |
| Concept / Comparison / Decision 页面 | 跨文档综合：概念解释、方案横向对比、技术决策记录（ADR 格式）。 |
| 本体图谱（Ontology Graph） | 编译过程中构建的类型化实体-关系图谱。Sage Wiki 内置 8 种关系类型（`implements`、`contradicts`、`trades_off` 等）。 |
| Schema / 模板 | 指导 LLM 编译风格的写作规则和页面模板，每个 KB 可自定义。 |

### 3. 索引与搜索

| 技术 | 说明 |
|---|---|
| SQLite FTS5 + BM25 | 核心搜索引擎——无需向量模型。亚毫秒级全文检索。WikiLoop、Sage Wiki、TreeSearch 均采用。 |
| 别名扩展 | 关键术语索引时内嵌别名和跨语言等价词，最大化 FTS 召回率。 |
| 图遍历 | `related_to` 链接实现多跳导航（类似 Wiki 页面链接）。搜索结果进行 BFS 扩展。 |
| 混合检索（FTS + 向量 + 图） | Sage Wiki：FTS5（411µs）+ 向量（81ms）+ 本体图谱（1µs），通过 RRF 融合。 |
| 章节树索引 | 保留文档 H1/H2/H3 层级——平铺切块的结构保留替代方案。 |

### 4. 知识质量与维护

| 技术 | 说明 |
|---|---|
| Git 版本控制 | 所有 Wiki 页面在 Git 中——完整历史、diff、blame，知识变更完全可审计。 |
| Lint / 健康检查 | 验证 frontmatter、断开的 source 链接、缺失引用。`wikiloop lint`。 |
| 冲突检测 | `contradicts` 链接将来源间的分歧暴露出来，供人工审核。 |
| Draft 暂存 | 来源少于 2 个的页面隔离到 `_draft/`——验证补充后才进入索引。 |
| 知识空白分析 | `wikiloop synthesize --gaps` 识别覆盖不足的主题。 |
| 实体去重 / 解析 | 识别同一概念的不同表述并合并为单一节点。 |

### 5. Agent 接口（MCP）

| 技术 | 说明 |
|---|---|
| [MCP 协议](https://modelcontextprotocol.io) | Model Context Protocol——Anthropic 开放标准，向 AI Agent 暴露工具/资源。支持 stdio + HTTP 双传输。 |
| `kb_search` | FTS 关键词搜索，返回带 `related` 链接的排序结果，用于图谱导航。 |
| `kb_page` | 通过 ID 获取完整页面内容，支持批量（最多 5 个 ID）或 `full=true` 获取不截断文本。 |
| MCP Resources | 只读资源（URI 形式）：Wiki 页面、图谱 Schema、原始文档。 |
| 迭代搜索模式 | Agent 从不同角度发出多次查询，跟随 `related` 链接，自行综合答案。 |
| Sage Wiki MCP 工具 | 17 个工具：6 读、9 写、2 复合——Agent 可直接写入和编译知识。 |

### 6. 文件转换（输入层）

| 技术 | 说明 |
|---|---|
| markitdown（Microsoft） | 将 PDF、Word、Excel、PPT、HTML 转为 Markdown 后再提炼。 |
| `raw/converted/` 模式 | Agent 提取的内容直接写入此目录——跳过转换，直接进入提炼流程。 |
| 文件监听器（Watcher） | 自动检测 `raw/` 下的新文件/变更文件，触发转换 → 提炼 → 索引全流程。 |

---

## 技术对比

| 维度 | RAG | LLM Wiki |
|---|---|---|
| 核心操作 | 查询时检索 | 写入时编译 |
| 存储 | 向量数据库 + 原始 chunk | 结构化 Markdown + SQLite FTS |
| 搜索方式 | ANN 相似度搜索 | FTS5 BM25 + 图遍历 |
| 知识形态 | 隐式（向量） | 显式（可读 Markdown） |
| 可审计性 | 低 | 高（git diff、lint） |
| 多跳推理 | 依赖 LLM | 通过 `related` 图链接 |
| 是否需要 Embedding | 是 | 否（纯 FTS） |
| 知识积累 | 无（静态索引） | 随时间复利增长 |
| 查询时 Token 成本 | 高（原始 chunk） | 低（预编译页面） |
| 最适合场景 | 宽泛文档问答、实时数据 | 长期知识管理、Agent 记忆、研究 |
