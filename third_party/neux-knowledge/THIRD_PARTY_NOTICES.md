# 第三方来源与许可

## Tencent WeKnora

本核心使用 [Tencent/WeKnora](https://github.com/Tencent/WeKnora) 固定提交 `8776a2d92d11ad3e76865d51cf3192e37e2a9976`。原完整许可逐字保存在 [upstream/WeKnora.LICENSE](upstream/WeKnora.LICENSE)，版权属于原权利人，原声明保持不变。真实文件、函数、原始与目标摘要、适配范围见 [总清单](upstream/manifest.json)、[管线清单](upstream/pipeline-manifest.json)、[核查清单](upstream/lint-manifest.json)。本地新增代码不替代上游声明。

直接迁移的主体包括 Wiki 模型与仓储、完整分块与来源定位、Pass0及旧extract回退、文档内去重、Lite身份保留与精确查询缓存、cite分批/句柄/归并、mapOneDocument、reduceSlugUpdates、目录树及向量筛选、图片URL冻结、JSON清洗、摘要/索引提示词、普通文档Profile解析与采样、重叠块重建、模糊slug修复、自动链接与表格行保护。相应原测试保留，并用真实SQLite和可控原生模型验证独立核心适配。

Wiki维护迁入同一固定提交的整页Agent写入、精确替换、重命名与删除工具规则及相关测试。SQLite适配保留稳定页面身份、归档历史、人工保护、真实来源校验和版本比较，页面及入链级联更新与修订、回执复用原事务，重命名同步保留的历史问题记录；实际来源与适配登记于总清单。

SQLite、受管原件、固定来源版本、请求幂等、权限与版本CAS、纯知识阶段诊断、显式整理的运行与停止、持久finalize增量及宿主资源通知属于本地适配。核心不依赖NeuxAlly internal；宿主通过直接模型端口提供CLI能力，不创建Waker、Chat、Execution或聊天JSONL。原Redis、多租户与Asynq服务不移植，Lite同库锁、批次和局部失败规则由独立仓储承接。显式Run待办收束后进行一次finalize，替代原20秒触发器防抖；不声称导入完整上游服务依赖。

Wiki相关原提示词与模型消息角色保持，Thinking=false、温度0.3、Wiki输出预算32768；普通文档Profile保持system/user角色、8192输入采样和1024输出预算。CLI的模型可用性、实际推理档位、工具关闭和无法保证的限制由宿主报告真实诊断，不向模型叠加旧文件提交或阶段技能协议。上游供应商singleflight/prefix warmup依赖供应商和租户模型接口，当前CLI端口不复制该缓存实现。

当前Reader只接入实际可用格式，不引入上游重型解析、图像OCR增强、普通摘要独立开关或完整post_process队列。SQLite相似候选查询使用已迁有界关键词/别名分支，精确身份查询保留原SQLite规范化逻辑；这不是PostgreSQL pg_trgm、BM25或语义检索。向量与关键词索引使用真实原始文档父子块；嵌入批次默认5条，顺序回填与首错停止保留，顺序端口调用替代ants池。

库活动、索引只读分页、核查、导出、来源和整库清理的适配边界分别见相关业务文档。WikiMesh没有迁入代码，也不作为运行依赖。

## Go依赖

锁定版本见go.mod/go.sum，已分发模块的原始许可与摘要见 [依赖许可清单](upstream/dependency-license-manifest.json)。google/uuid为BSD-3-Clause，纯Go SQLite保留BSD、public-domain与libc组合声明。根LICENSE适用于本项目新增部分，不取代这些许可。

核心采用glebarez/sqlite的sqlite驱动名，不注册宿主sqlite3驱动；不引入Redis、外部向量服务、模型供应商SDK或完整解析服务。实际二进制依赖以go list -deps为准。
