# 评估能力（Evaluation）

评估使用带标准答案的问答数据集，比较分块、模型和检索配置的效果。系统创建评估知识库并导入语料，逐题执行检索与生成，输出 Precision、Recall、NDCG、MRR、MAP、BLEU 和 ROUGE 等指标。

::: tip 评测工作台
侧边栏选择「评测」打开 `/platform/evaluations`。工作台提供数据集导入与编辑、草稿和发布版本、发起评测、历史记录、逐题答案、执行过程、指标和结果导出；也可以继续通过 API 使用。
:::

比较配置时应固定数据集及指标版本，每次调整一个变量。检索排序分和评测质量分含义不同，不能直接互相比作正确率。

## 运行一次评估

1. 在工作台「数据集」中新建或导入数据集，也可以复制只读的内置样例。
2. 编辑问题、参考答案和证据，保存草稿并发布版本。内置 `default` 仍直接从 Parquet 文件读取。
3. 点击发起评测，选择已发布的数据集版本、参考知识库和模型。参考知识库用于复制配置，语料在单独的临时知识库中评测。
4. 在「评测历史」中查看状态、进度和汇总指标，进入运行详情查看逐题结果及配置快照。
5. 展开一道题，比较参考答案和最终答案，查看实际执行的节点、输入输出、段落排序分和节点评测指标。失败时查看具体错误。

创建、编辑、发布数据集及发起评测需要 Admin；查看数据集及结果需要 Viewer。API Key 的数据集写操作需要 full-access；读取及发起评测可以使用 `run_evaluations` 或 full-access。评测属于工作空间级功能，知识库受限的 API Key 不可访问。

## 在界面维护数据集

「数据集」支持新建、文件导入、编辑草稿、发布版本和查看版本历史。

- **草稿**：可增删题目、修改问题和标准答案、维护语料正文、选择题目引用的证据。可以保存尚未填完的内容；ID 和引用结构必须有效。
- **共享证据**：题目通过 PID 引用全局语料。修改正文会影响所有引用它的题目，界面显示引用数量。仍被引用的证据不能删除，需先取消相关题目的引用。
- **发布**：至少有一道题；每道题必须有问题、标准答案及证据；语料正文不能为空，完全相同的正文应共用一个 PID。发布后生成只读的 `v1`、`v2` 等版本，完整保存题目、全部语料及元信息。
- **历史恢复**：查看旧版后可将其恢复为当前草稿，之后另行保存和发布。恢复不会覆盖旧版本或改变当前已发布的版本。
- **并发编辑**：保存和发布都带草稿 revision。旧页面的 revision 已过期时返回 409，保留本地编辑，需重新加载后核对，不自动覆盖其他人的修改。
- **内置样例**：`default` 只读，可复制成自己的可编辑数据集。复制和 JSON 导出保留全部语料，包括未被题目引用的干扰段落。

开始评测只接受已发布版本，不能直接执行草稿。未指定版本时使用最新发布版；指定 `v1` 等旧版时固定使用该版。后台任务使用创建时已冻结的快照，之后保存草稿或发布新版均不会改变该次评测。

### 文件导入

JSON 为完整内容格式，保留显式 ID、共享引用和干扰语料：

```json
{
  "questions": [
    {"qid": 1, "question": "评测历史会在服务重启后丢失吗？", "answer": "不会，结果已持久化到数据库。", "pids": [10]}
  ],
  "corpus": [
    {"id": 10, "text": "评测结果保存在数据库 evaluation_runs 表中，服务重启后仍可查询。"},
    {"id": 20, "text": "工作台可预览评测题目和关联证据。"}
  ]
}
```

JSONL 为简易题目格式，每行一个对象：

```jsonl
{"question":"评测历史会丢失吗？","answer":"不会，已持久化。","evidence":["结果保存在数据库中，服务重启后仍可查询。"]}
```

CSV 必须包含 `question`、`answer`、`evidence` 列；每格证据是一段完整原文，需要更多证据时增加 `evidence_2`、`evidence_3` 等列。逗号、引号和多行正文遵循标准 CSV 引号规则。工作台提供可下载模板。

JSONL 和 CSV 导入时自动分配整数 QID/PID，并将相同正文转换为共享证据。它们只包含文件提供的题目和证据；需要保存未引用语料或已有 ID 时应使用完整 JSON 格式。

导入先展示预览，经确认后整体保存为新草稿。无效记录会报告位置和原因并拒绝整个导入，不会跳过坏行或部分保存。错误过多时会限量显示诊断并明确整体拒绝，避免错误列表本身占用过多资源。文件内容只作为文本处理。

规范内容最多 8 MiB，最多 1000 道题、5000 个语料段落；单个文本字段最多 65536 UTF-8 字节。发布或导入时，所有题目引用的证据正文累计展开后也不能超过 8 MiB，同一段被多题引用时按每次引用计入。名称最多 200 个字符，描述最多 2000 个字符。HTTP 请求上限为 10 MiB，包含元信息；超过限制会明确失败。

## 内置 Parquet 数据集

数据集服务（`internal/application/service/dataset.go`）从 `./dataset/samples/` 加载 5 个 **Parquet** 文件：

| 文件 | Schema | 含义 |
| --- | --- | --- |
| `queries.parquet` | `id: int64, text: string` | 问题集合 |
| `corpus.parquet` | `id: int64, text: string` | 语料段落（评估时灌入知识库） |
| `answers.parquet` | `id: int64, text: string` | 参考答案 |
| `qrels.parquet` | `qid: int64, pid: int64` | 问题 → 相关段落的 ground truth 关联（检索指标依据） |
| `qas.parquet` | `qid: int64, aid: int64` | 问题 → 答案映射（生成指标依据） |

对应的 Go 结构体：

```go
type TextInfo struct {
    ID   int64  `parquet:"id"`
    Text string `parquet:"text"`
}
type RelsInfo struct {
    QID int64 `parquet:"qid"`
    PID int64 `parquet:"pid"`
}
type QaInfo struct {
    QID int64 `parquet:"qid"`
    AID int64 `parquet:"aid"`
}
```

加载后拼装为逐样本的 `QAPair`（`internal/types/dataset.go`）：

```go
type QAPair struct {
    QID      int      // 问题 ID
    Question string   // 问题文本
    PIDs     []int    // 相关段落 ID（ground truth）
    Passages []string // 段落文本
    AID      int      // 答案 ID
    Answer   string   // 参考答案文本
}
```

内置 `dataset_id = "default"` 从 `./dataset/samples/` 读取这 5 个文件（Docker 可挂载到 `/app/dataset/samples/`）；它不属于数据库草稿，不支持直接编辑。自建数据集保存到数据库，使用各自的 ID 和已发布版本。内置数据文件变化后，旧哈希不可读取时会返回不存在错误，已有评测仍保留当时的结果快照。

## 结果查询

`GET /api/v1/evaluation?task_id=<创建时返回的任务 ID>`，返回 `EvaluationDetail`：

```json
{
  "success": true,
  "data": {
    "task": {
      "id": "evaluation_1_1758600000000_1a2b3c4d_default",
      "dataset_id": "default",
      "status": 2,
      "total": 100,
      "finished": 100
    },
    "params": { "...": "ChatManage 评估参数快照" },
    "metric": {
      "retrieval_metrics": {
        "precision": 0.85, "recall": 0.92,
        "ndcg3": 0.88, "ndcg10": 0.86,
        "mrr": 0.95, "map": 0.87
      },
      "generation_metrics": {
        "bleu1": 0.72, "bleu2": 0.65, "bleu4": 0.58,
        "rouge1": 0.78, "rouge2": 0.71, "rougel": 0.75
      }
    }
  }
}
```

任务运行期间可轮询该接口获取 `finished / total` 进度；`status = 3` 时 `err_msg` 携带失败原因。

结果、题目和配置快照保存到数据库的 `evaluation_runs` 表，服务重启后仍可查询。历史展示运行当时的题目、参考答案和输出，不会重新读取最新数据集。每个运行每 15 秒续租，超过 90 秒没有心跳的未完成任务在查询时标记为中断失败，不自动重跑。未完成题目与执行错误单独记录；汇总指标仅统计已成功评分的题目，需结合失败数解读。

## 指标清单

指标注册表见 `internal/application/service/metric_hook.go`，共 12 项，分两组。文本先经 `metric/common.go` 分词：中文用 Jieba 分词、英文按空白切分、按 `。` / `.` 切句。

### 检索指标（Retrieval Metrics）

| 指标 | 字段 | 实现文件 | 含义 |
| --- | --- | --- | --- |
| Precision | `precision` | `metric/precision.go` | 检索准确率：命中的相关文档数 / 检索返回总数，按 GT 集合求均值 |
| Recall | `recall` | `metric/recall.go` | 检索召回率：命中的相关文档数 / 相关文档总数 |
| NDCG@3 | `ndcg3` | `metric/ndcg.go` | 归一化折损累计增益（取前 3 位），奖励把相关文档排在前面 |
| NDCG@10 | `ndcg10` | `metric/ndcg.go` | 同上，取前 10 位 |
| MRR | `mrr` | `metric/mrr.go` | 首个相关文档倒数排名的平均：`sum(1/rank) / N` |
| MAP | `map` | `metric/map.go` | 平均精度均值：对每个命中位置累计 `Precision@k` 再归一化 |

NDCG 核心计算（`metric/ndcg.go`）：

```go
// DCG = sum((2^rel_i - 1) / log2(i+2))，rel 为 0/1
dcg += (math.Pow(2, float64(relevance)) - 1) / math.Log2(float64(i+2))
// NDCG = DCG / IDCG（理想排序的 DCG）
```

MRR 核心计算（`metric/mrr.go`）：

```go
for i, predID := range ids {
    if _, ok := gtSet[predID]; ok {
        sumRR += 1.0 / float64(i+1) // 第一个命中位置的倒数
        break
    }
}
```

### 生成指标（Generation Metrics）

| 指标 | 字段 | 实现文件 | 含义 |
| --- | --- | --- | --- |
| BLEU-1 | `bleu1` | `metric/bleu.go` | 1-gram 精度（权重 `[1.0, 0, 0, 0]`） |
| BLEU-2 | `bleu2` | `metric/bleu.go` | 1/2-gram 各 50%（权重 `[0.5, 0.5, 0, 0]`） |
| BLEU-4 | `bleu4` | `metric/bleu.go` | 1~4-gram 均权（`[0.25, 0.25, 0.25, 0.25]`），含 brevity penalty |
| ROUGE-1 | `rouge1` | `metric/rouge.go` | 一元词重叠 F1 |
| ROUGE-2 | `rouge2` | `metric/rouge.go` | 二元词组重叠 F1 |
| ROUGE-L | `rougel` | `metric/rouge.go` | 最长公共子序列（LCS）F1 |

BLEU 核心（`metric/bleu.go`）：修正 n-gram 精度的加权几何平均乘以简短惩罚 `bp * exp(sum(w_i * log(p_i)))`。ROUGE 取 F1：`F1 = 2PR / (P + R + 1e-8)`（`metric/rouge_score.go`）。

## 执行过程与评分

逐题详情保存实际执行的检索、重排、段落合并、上下文构建和生成步骤。只有实际运行的步骤才会出现；没有命中时，还会记录实际执行的兜底响应。每个步骤包含状态、耗时、输入输出及当时的段落快照，后续阶段修改排序分不会改写前面的记录。

- **段落排序分**：来自该阶段的检索或重排结果，用于解释排序，不等于回答正确率。
- **检索评测分**：以检索或重排阶段输出的段落与人工证据关联计算 Precision、Recall、NDCG、MRR、MAP。整题检索指标采用重排后的排名；未执行重排时采用检索排名。
- **生成评测分**：以最终答案和参考答案计算 BLEU、ROUGE。它们衡量文字重合，不能代替事实正确性、忠实度或拒答正确性。
- **未评分步骤**：段落合并、上下文构建等没有相应评分器的步骤显示未评分；失败步骤保留错误，不把执行失败伪装成质量零分。

评测会导入所选版本的完整语料（内置数据来自 `corpus.parquet`），包括未被题目引用的干扰段落。检索结果通过正文匹配映射回数据集 passage ID，未匹配或不相关结果仍参与精确率和排序计算。当前指标版本为 `corpus-passage-v3`：MAP 按全部相关证据数归一化，NDCG 的理想排序长度为 `min(K, 相关证据数)`，遗漏证据会降低得分。正文匹配属于当前实现的评测口径；相同或相互包含的段落可能存在映射歧义，不能将这些结果直接等同于生产全库指标。

题目没有参考答案或证据时，当前流程会明确拒绝启动。无答案题需要专门的拒答评分器，当前尚未提供。

## API

所有响应沿用 `{ "success": true, "data": ... }`。以下路径均以 `/api/v1` 为前缀。

| 方法 | 路径 | 权限 | 说明 |
| --- | --- | --- | --- |
| GET | `/evaluation/datasets` | Viewer | 可用数据集的 ID、名称、内容版本及题数 |
| GET | `/evaluation/datasets/:id?version=v1` | Viewer | 指定已发布版本；省略 version 取最新；返回全部语料 |
| POST | `/evaluation/datasets` | Admin | 创建新草稿，含名称、描述和完整内容 |
| GET | `/evaluation/datasets/:id/draft` | Viewer | 当前草稿及 revision |
| PUT | `/evaluation/datasets/:id/draft` | Admin | 保存草稿，必须携带当前 revision |
| POST | `/evaluation/datasets/:id/publish` | Admin | 发布已保存草稿，必须携带当前 revision |
| GET | `/evaluation/datasets/:id/versions` | Viewer | 只读发布版本列表及内容哈希 |
| POST | `/evaluation` | Admin | 创建并异步执行评测，返回任务详情 |
| GET | `/evaluation/runs?page=1&page_size=20` | Viewer | 当前工作空间的分页评测历史 |
| GET | `/evaluation?task_id=...` | Viewer | 运行快照、进度、汇总指标及逐题执行记录 |

创建参数：

```json
{
  "dataset_id": "自建数据集 ID 或 default",
  "dataset_version": "v1，可省略；default 可指定当前内容哈希",
  "knowledge_base_id": "参考知识库 ID，可省略",
  "chat_id": "对话模型 ID，可省略",
  "rerank_id": "重排模型 ID，可省略"
}
```

未指定模型时按当前模型配置选择，并保存实际使用的模型 ID。参考知识库只提供配置，评测不会向其写入数据。任务 ID 每次创建均不同，应使用返回的 `task.id` 查询。

发布序号与内容哈希分开保存：`task.dataset_version_label` 是所选发布序号，`task.dataset_version` 是内容哈希，运行对比仍依据内容哈希及指标版本。

详情沿用 `task`、`params`、`metric`，并增加 `results`：每题包含问题、参考答案、最终答案、状态、耗时、错误、模型用量（有返回时）、评测指标和 `steps`。步骤的 `metric_scope` 区分检索与生成指标；没有指标时不返回该字段。

任务状态沿用 `0` 待启动、`1` 执行中、`2` 成功、`3` 失败。逐题状态为 `pending`、`running`、`success` 或 `failed`。运行详情可按失败、低分、关键词筛选并导出为 JSON；相同数据集版本与指标版本的成功运行可以对比汇总指标和逐题最终答案。

## 实现参考

| 层 | 文件 |
| --- | --- |
| 工作台 | `frontend/src/views/evaluation/` |
| 前端 API | `frontend/src/api/evaluation.ts` |
| HTTP Handler 与路由 | `internal/handler/evaluation.go`、`internal/router/routes_infra.go` |
| 任务执行 | `internal/application/service/evaluation.go` |
| 数据库存储 | `internal/application/repository/evaluation.go` |
| 流程追踪 | `internal/application/service/pipeline_trace.go`、`session_knowledge_qa.go` |
| 指标与汇聚 | `internal/application/service/evaluation_metrics.go`、`metric_hook.go`、`metric/` |
| 数据集与类型 | `internal/application/service/dataset.go`、`internal/types/evaluation.go` |
| 样例数据与制作工具 | `dataset/samples/`、`dataset/qa_dataset.py` |

业界功能依据和后续扩展方向见[评测工作台调研记录](../../docs/research/evaluation-workbench.md)。
