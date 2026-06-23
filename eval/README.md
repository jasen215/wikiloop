# WikiLoop 评估框架

用 RAGAS 四项指标对比 WikiLoop 与 Naive RAG 的检索质量。

## 指标说明

| 指标 | 含义 |
|---|---|
| Faithfulness（忠实度） | 答案是否来自检索内容，不 hallucinate |
| Answer Relevancy（答案相关性） | 答案是否回答了问题 |
| Context Precision（上下文精度） | 检索内容中有用的比例 |
| Context Recall（上下文召回） | 相关内容是否被检索到 |

## Baseline（2026-06-20，无 Rerank/Pre-Route）

问题集：`questions_rag.json`（10 题，RAG/LLM Wiki 主题）

| 指标 | WikiLoop | Naive RAG | 提升 |
|---|---|---|---|
| 忠实度 | 0.825 | 0.900 | ↓ -7.5% |
| 答案相关性 | 0.660 | 0.425 | ↑ +23.5% |
| 上下文精度 | 0.122 | 0.060 | ↑ +6.2% |
| 上下文召回 | 0.360 | 0.085 | ↑ +27.5% |

**检索链路**：FTS + Vector → RRF 融合 → 直接返回（无 Rerank，无 Pre-Route）

**结论**：WikiLoop 在答案相关性（+23.5%）和上下文召回（+27.5%）上显著优于 Naive RAG。
上下文精度（0.122）是主要优化方向，预期 Rerank 实现后会有明显提升。

详细结果：`baseline_result.json`

## 运行评估

```bash
# 依赖
pip install ragas requests pyyaml

# 使用预设问题集（推荐，可复现对比）
cp eval/questions_rag.json /tmp/eval_questions.json
python3 eval/eval_wikiloop.py

# 自动从 KB 生成问题集
rm /tmp/eval_questions.json
python3 eval/eval_wikiloop.py
```

**前置条件**：
- WikiLoop serve 已启动（`wikiloop serve`）
- `~/.hermes/wikiloop-kb/config.yaml` 中已配置 LLM（distill 段）

## 对比历史

| 日期 | 版本/变更 | 答案相关性 | 上下文精度 | 上下文召回 |
|---|---|---|---|---|
| 2026-06-20 | Baseline（RRF，无 Rerank） | 0.660 | 0.122 | 0.360 |
| 2026-06-20 | Phase 1（Bi-Encoder Rerank + 短文档不分块） | 0.520 | 0.186 | 0.320 |
| 2026-06-20 | 实验：关闭分块（整文档嵌入）+ Rerank | 0.670 | 0.120 | 0.380 |
| 2026-06-20 | 实验：MMR 多样性约束 + 分块 + Rerank | 0.625 | 0.151 | 0.300 |
| **结论** | **Phase 1 检索优化未突破 baseline，wiki 覆盖度（1.7%）是根本瓶颈，需推进 Phase 2** | | | |
| 2026-06-20 | Phase 2（去阈值 synthesize，12→18 篇综合页）| 0.610 | 0.282 | 0.390 |
| 2026-06-21 | Phase2完整(933篇) + kind过滤 + 引用强制 + 上下文压缩 + limit=10 | 0.630 | 0.289 | 0.260 |
| 2026-06-21 | + 压缩优化（Jaccard 0.5 + 低质量综合页过滤 description<30字） | 0.510 | 0.324 | 0.300 |
| 2026-06-21 | + AND-first FTS + graph_pages wiki-only + minHybridScore=0.025 | 0.720 | 0.206 | 0.330 |
| （待补充） | Phase 3（实体索引） | — | — | — |

## v2 问题集结果（concept/comparison/decision，含 Hit Rate + MRR）

| 版本 | AR | CP | CR | Hit Rate | MRR |
|---|---|---|---|---|---|
| 933篇综合页全量嵌入 | 0.992 | 0.411 | 0.467 | 0.083 | 0.017 |
| **只嵌入469篇source-notes（当前最佳）** | **0.988** | **0.455** | **0.567** | **0.167** | **0.095** |
| 2026-06-22 有向量 baseline（v2题集，新评估环境）| 0.950 | 0.511 | 0.633 | 0.167 | 0.097 |
| 2026-06-22 **无向量 FTS-only**（WIKILOOP_NO_VEC=1）| 0.992 | 0.495 | 0.542 | 0.167 | 0.097 |

**向量搜索贡献分析（2026-06-22）：**
- CR 差距：0.633 → 0.542，**下降 0.091（14%）**，向量有真实贡献
- CP 差距：0.511 → 0.495，**下降 0.016（3%）**，几乎可忽略
- Hit Rate / MRR：完全一样，向量对精确页面定位无帮助
- 注意：此为旧文档未重蒸馏的**最坏情况**，全量重蒸馏加入 ALIAS RULE 后差距预计缩小

**最终决策（2026-06-23）：放弃向量搜索。**
全量重蒸馏（ALIAS RULE + ENTITY RULE + authority + doc_type）后，有向量 CR=0.433，无向量 CR=0.433，差距为零。
向量不仅无贡献，Hit Rate 和 AR 无向量反而更高——向量把高 vec_score 的无关 source-note 顶上来，把真正相关的 comparison/decision 页挤出了结果集。
下一步：执行 15.3，删除 chromem-go + daulet/tokenizers，解决内存和 Windows CGO 问题。

## v2 问题集最终结果（2026-06-23，全量重蒸馏后）

| 版本 | AR | CP | CR | Hit Rate | MRR |
|---|---|---|---|---|---|
| 有向量（修复 multiKindFTS + expected_page）| 0.904 | 0.569 | 0.433 | 0.000 | 0.000 |
| **无向量（FTS only，同上修复）** | **0.988** | **0.492** | **0.433** | **0.250** | **0.041** |

**结论：CR 完全持平，AR/Hit Rate 无向量更优，确认放弃向量搜索。**

## 问题集说明

`questions_rag.json`：10 道 RAG/LLM Wiki 主题问题，ground truth 来自 KB 中高质量 source-notes，覆盖：
- RAG 优化方法（五阶段、RRF、HyDE、Rerank）
- 向量数据库选型（ChromaDB/FAISS/Qdrant/Milvus）
- UnWeaver vs GraphRAG
- LLM Wiki vs 传统 RAG（Karpathy 三层架构）
