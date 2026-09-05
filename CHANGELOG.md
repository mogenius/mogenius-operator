# Changelog

## [2.30.0](https://github.com/mogenius/mogenius-operator/compare/v2.29.0...v2.30.0) (2026-09-05)


### Features

* add GroupGrant rule CRD with socket patterns and spec-carrying cluster events ([327228a](https://github.com/mogenius/mogenius-operator/commit/327228a6ad5db41661b88cdd20fc62d9e36fdea9))
* add oci helm cart upgrade path ([019c082](https://github.com/mogenius/mogenius-operator/commit/019c0821363b3bb1dbe8c2c9e6ca68f0655820b2))
* allow helm repo unlinking ([226b685](https://github.com/mogenius/mogenius-operator/commit/226b685f2f9b72e939ee0e3c3da416361ef662dd))
* show set values for oci helm releases ([c9b727e](https://github.com/mogenius/mogenius-operator/commit/c9b727e71101398fb7be3245c5cf659a1bca61d3))
* SSH, scp and port forwarding without sshd ([cdf8c6f](https://github.com/mogenius/mogenius-operator/commit/cdf8c6f44884606c8fcf20de6a3550f7b1a7be71))
* **storage:** expose helper pod status in storage/v2/info ([ba4f6a3](https://github.com/mogenius/mogenius-operator/commit/ba4f6a33653ca005db8595e8156baf4730bde768))
* **storage:** files/v2/search pattern and createdAt in storage/v2/info ([4417b0d](https://github.com/mogenius/mogenius-operator/commit/4417b0d89cd3da9c69dab3de40439cd4ed69896e))
* **storage:** generic PVC exec substrate and storage/files v2 patterns ([05a72d4](https://github.com/mogenius/mogenius-operator/commit/05a72d40310efa6b6fee85e92416e215f4fc87ea))
* **storage:** helper mounter pod for unmounted PVCs ([e4f45d2](https://github.com/mogenius/mogenius-operator/commit/e4f45d24d27f268173cfc40d0b34a0538ceb6b07))


### Bug Fixes

* **agent:** fail the agent if compaction fails ([78267e6](https://github.com/mogenius/mogenius-operator/commit/78267e6c57a7994bb3392d27cdb83a6bbe2d0fbc))
* **agent:** improve agent label when reasoning is finished ([8c9a2b4](https://github.com/mogenius/mogenius-operator/commit/8c9a2b4fe302bd6e6b23bf082416ef928be7601e))
* **agent:** improve budget tests and error handling ([b04bde9](https://github.com/mogenius/mogenius-operator/commit/b04bde90eb6e279c856e35537243184168ac71d7))
* **agent:** improve message if agent hit its context limits ([fa41889](https://github.com/mogenius/mogenius-operator/commit/fa4188950bb189252236a05acc00f1d337fe6deb))
* agents can only be scoped on a single workspace and listed by workspace ([a8fbce7](https://github.com/mogenius/mogenius-operator/commit/a8fbce76504795e39844d91bd756e41ba91b53db))
* **agents:** adding warning if ai response lenght isnt readable ([714b859](https://github.com/mogenius/mogenius-operator/commit/714b859bb8f662425400df47e06cbfc3d7312158))
* container started might not yet be set ([1a1bf0d](https://github.com/mogenius/mogenius-operator/commit/1a1bf0d2d680c492339320701caacca269decbb7))
* **deps:** update kubernetes monorepo to v0.37.0 ([#1197](https://github.com/mogenius/mogenius-operator/issues/1197)) ([4d2d4d5](https://github.com/mogenius/mogenius-operator/commit/4d2d4d5fd46fca2e83015a7f8913d12abad62d31))
* **deps:** update module github.com/alicebob/miniredis/v2 to v2.39.0 ([9530604](https://github.com/mogenius/mogenius-operator/commit/95306048c9846601404bba5f9c107e87b9b3a9bd))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.68.0 ([f422eb8](https://github.com/mogenius/mogenius-operator/commit/f422eb83a8d66318060dfe87fb415a5a925f8dfb))
* **deps:** update module github.com/bitnami/sealed-secrets to v0.39.1 ([#1194](https://github.com/mogenius/mogenius-operator/issues/1194)) ([2cfd838](https://github.com/mogenius/mogenius-operator/commit/2cfd83895e83873534d9f3c2790a208964056e35))
* **deps:** update module github.com/go-playground/validator/v10 to v10.30.4 ([#1219](https://github.com/mogenius/mogenius-operator/issues/1219)) ([31dbf4f](https://github.com/mogenius/mogenius-operator/commit/31dbf4fd96fdc4bbdc299e62b385624e4029312d))
* **deps:** update module github.com/kimmachinegun/automemlimit to v1 ([cb782a2](https://github.com/mogenius/mogenius-operator/commit/cb782a2426bf77a057ea4558216125024c5efa3e))
* **deps:** update module github.com/ollama/ollama to v0.33.2 ([adafe21](https://github.com/mogenius/mogenius-operator/commit/adafe21c08c13259d7e7e939ba3c8427759c455e))
* **deps:** update module github.com/openai/openai-go/v3 to v3.54.0 ([#1200](https://github.com/mogenius/mogenius-operator/issues/1200)) ([2f6497a](https://github.com/mogenius/mogenius-operator/commit/2f6497a9fb148217446c07fc8616ddfeb4a2eeed))
* **deps:** update module github.com/openai/openai-go/v3 to v3.56.0 ([#1207](https://github.com/mogenius/mogenius-operator/issues/1207)) ([52e4597](https://github.com/mogenius/mogenius-operator/commit/52e4597787eb5acfb91420fcc79ac5cd6aba268a))
* **deps:** update module golang.org/x/crypto to v0.56.0 ([#1210](https://github.com/mogenius/mogenius-operator/issues/1210)) ([07e1951](https://github.com/mogenius/mogenius-operator/commit/07e19513dfa1aa05a02717da4fdc6facf7c0d4c2))
* do not collect metrics if node does not have an adress ([32e1176](https://github.com/mogenius/mogenius-operator/commit/32e1176dbdbc1fa00b9e1565f91f44e5d8e586f5))
* **flux:** annotate the generated HelmChart on reconcile-with-source ([26af765](https://github.com/mogenius/mogenius-operator/commit/26af7652d1022f8cf74a18f0a1f86044de7d38aa))
* if chat call failed mark the step as failed ([fbc12bb](https://github.com/mogenius/mogenius-operator/commit/fbc12bbd2ca5b36eb6e2d1c6e4d4ea249560ff1f))
* improve validation for workspace ref ([cccc183](https://github.com/mogenius/mogenius-operator/commit/cccc183f0df811974c47f9dc7353f69682f0b6b5))
* remove inlining to reduce cpu load ([6886f6b](https://github.com/mogenius/mogenius-operator/commit/6886f6b50aba32df38f0174c45f0426e147f1e52))
* renovate for busybox incode stuff ([f8251a2](https://github.com/mogenius/mogenius-operator/commit/f8251a202d8b101b146b721b3ce1ef4cffe1873d))
* set max ai response lenght to 8000 chars ([e9b95d4](https://github.com/mogenius/mogenius-operator/commit/e9b95d4371fed8003091a189bd4b391d12c847b3))
* **storage:** handle file uploads on every API connection ([720f0ba](https://github.com/mogenius/mogenius-operator/commit/720f0baab565c58772cd45bffe3c8a337c595577))
* **storage:** ignore terminating pods for mount state and exec targets ([d549099](https://github.com/mogenius/mogenius-operator/commit/d5490998e50efc0482c8b5633f8ba8728a458edb))
* **storage:** portable df (-P -k) and position-independent parsing ([42f727c](https://github.com/mogenius/mogenius-operator/commit/42f727ccb266618671c3a5105d08d07094e16781))
* **storage:** reap helper pods of terminating PVCs ([e6e347b](https://github.com/mogenius/mogenius-operator/commit/e6e347bb7f0633fcb950d70d41bc335028b1c2d1))
* workspace selector mismatch for agents ([86abcd4](https://github.com/mogenius/mogenius-operator/commit/86abcd4d9db60bf6388aa7e396855f8ada01bb63))

## [2.29.0](https://github.com/mogenius/mogenius-operator/compare/v2.28.0...v2.29.0) (2026-08-21)


### Features

* **ai:** adding new ai sdk ([31f83df](https://github.com/mogenius/mogenius-operator/commit/31f83df78df90d04cd9a04ed056cb8c944755c26))
* allow retrieving of oci helm chart versions ([940ac5e](https://github.com/mogenius/mogenius-operator/commit/940ac5e11b293920d7b620f8d4f3e17870726826))
* migrate agent and chat loop to the new ai sdk ([54bdf18](https://github.com/mogenius/mogenius-operator/commit/54bdf18059acae99bc8abd32a4e1c696d63fe5a4))


### Bug Fixes

* **agent:** try to get model context and compat at 80% of it ([3b8ea9f](https://github.com/mogenius/mogenius-operator/commit/3b8ea9f1ab36012a0609e761251757d557befc20))
* **aisdk:** tools without parameters fail ([a0df6f8](https://github.com/mogenius/mogenius-operator/commit/a0df6f842922eecdce11c37473b92a1cf895eadc))
* **chat:** early return if budged is exceeded ([6044e4a](https://github.com/mogenius/mogenius-operator/commit/6044e4a61102b896a28a3af739e4b8e78e73a132))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.66.0 ([926be0d](https://github.com/mogenius/mogenius-operator/commit/926be0d284f6116694d3b44a2bacf61bb9c006ae))
* **deps:** update module github.com/bitnami/sealed-secrets to v0.39.0 ([b086a45](https://github.com/mogenius/mogenius-operator/commit/b086a45a730368ad8c6b5cb1d6c97674133522b7))
* **deps:** update module github.com/ollama/ollama to v0.32.15 ([f6be29b](https://github.com/mogenius/mogenius-operator/commit/f6be29b6cc95b29b15b2ca8ac9ce400d45c00235))
* **deps:** update module github.com/openai/openai-go/v3 to v3.52.0 ([5b901d3](https://github.com/mogenius/mogenius-operator/commit/5b901d33134db643bf31b8b4ddf1d64a765f4d2e))
* **deps:** update module github.com/stretchr/testify to v1.12.1 ([5086f8e](https://github.com/mogenius/mogenius-operator/commit/5086f8e71d28efe5a92d5bc93539be1350a27388))
* improving step recorder with step status ([dcd7e55](https://github.com/mogenius/mogenius-operator/commit/dcd7e55d9b3444088e7467ad64d77dcde40ce0ef))
* minor improvements ([c9d76e6](https://github.com/mogenius/mogenius-operator/commit/c9d76e6e201ecb6c421d8915fb3fcf1fe1b19322))
* operator panic on chat stream close ([12f00ba](https://github.com/mogenius/mogenius-operator/commit/12f00ba340048a239fa71e28624d2ad6e12e42bf))
* ran go fix for multiple best practice improvements ([74834ce](https://github.com/mogenius/mogenius-operator/commit/74834ce18c09f28e48e900d5ba2530793b3d6e50))
* surface errors if model hit its token output limit ([7546607](https://github.com/mogenius/mogenius-operator/commit/7546607619e6d2a210f604ae79e1446063cc3b3c))

## [2.28.0](https://github.com/mogenius/mogenius-operator/compare/v2.27.1...v2.28.0) (2026-08-19)


### Features

* **ai:** probe an ad-hoc model spec, not only the stored one ([90cb6fe](https://github.com/mogenius/mogenius-operator/commit/90cb6febb69b8f1516c749c2d0307af12d0190ec))
* **gitops:** declare gitops write access in PlatformConfig ([239b79a](https://github.com/mogenius/mogenius-operator/commit/239b79a37876ce535c8730991c2be84372372877))
* **gitops:** detect and report the installed gitops engine in platformconfig status ([b833ef2](https://github.com/mogenius/mogenius-operator/commit/b833ef2a86a84bf266e7af9a683246b90b132283))
* **gitops:** flux commands, workspace support, helm tagging and ai tools ([08159a1](https://github.com/mogenius/mogenius-operator/commit/08159a1d22baa3c488649aeb1f46f89aeb657aeb))
* report node region if set via label ([518b5c7](https://github.com/mogenius/mogenius-operator/commit/518b5c7459cfaa5d6c851d0fd7b7601c26c56421))
* report the status a failed request deserves ([ce3ff4e](https://github.com/mogenius/mogenius-operator/commit/ce3ff4e1d9ade74624c26346bc87265a1c8f951a))


### Bug Fixes

* added missing just generate output ([e8ea8f2](https://github.com/mogenius/mogenius-operator/commit/e8ea8f26361138c2fbecbe530378ef23dabab077))
* **agents:** pass the changed object into the prompt for event driven agents ([3a26410](https://github.com/mogenius/mogenius-operator/commit/3a264101e9be98fffe39e84a9b9a29f742bb4586))
* **ai:** add option to specify the legnth of ai response messages ([b2b5157](https://github.com/mogenius/mogenius-operator/commit/b2b5157104a42877948636553cc396c071293699))
* **ai:** ai driven compaction ([4f989dd](https://github.com/mogenius/mogenius-operator/commit/4f989dde0888686293e3b6b13a72ebdde22813d6))
* **ai:** anthropic compaction should use all text blocks ([9d4224f](https://github.com/mogenius/mogenius-operator/commit/9d4224f5a7c6558023f403dec78ed545a43cd2b1))
* **ai:** do not change calling messages array ([3152b49](https://github.com/mogenius/mogenius-operator/commit/3152b49fb99534fd339d1d12ac5dfa3d8691af6f))
* **ai:** removed old tool compaction and added error step recording ([5ff19e5](https://github.com/mogenius/mogenius-operator/commit/5ff19e506b1b1a8b7c9e109c7285d7bd984f7d7b))
* **ai:** tool call errors are not audit logged ([39003e0](https://github.com/mogenius/mogenius-operator/commit/39003e02dc3088eabd47b9f7f461014fc58a205b))
* **ai:** unify tool call handling and definition ([043f903](https://github.com/mogenius/mogenius-operator/commit/043f903d3c38ac981d7409d714aacdfa64eb3a85))
* allow adding typed config values ([39e288a](https://github.com/mogenius/mogenius-operator/commit/39e288aafe77ff69afe6ec97fa52efc4f50e1ab5))
* better surface budget errors to the user ([93bd37d](https://github.com/mogenius/mogenius-operator/commit/93bd37deddb50d82e1cfcc5c8eeb09b2c40d3a52))
* cancel concurrent builds. only the newest one should run. ([f84f50b](https://github.com/mogenius/mogenius-operator/commit/f84f50b9e41559e54efc6428a23b84b226a55f60))
* **cluster:** report node Ready condition in node stats ([00dd06f](https://github.com/mogenius/mogenius-operator/commit/00dd06f8288644d833608466c6ba1ca63337d7b4))
* delete not needed certmanager package reference ([d419448](https://github.com/mogenius/mogenius-operator/commit/d419448b60f6f5b6e120cb25d7c8a5584e5244eb))
* **deps:** update module github.com/alecthomas/kong to v1.16.1 ([90f1b23](https://github.com/mogenius/mogenius-operator/commit/90f1b23c7f72f7e952a99fdea9e1d492072aaac4))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.62.0 ([1525577](https://github.com/mogenius/mogenius-operator/commit/152557717e032d9bbe9a0a65e2da2f7f5df4979a))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.63.0 ([b1e72f2](https://github.com/mogenius/mogenius-operator/commit/b1e72f2478b8b0a205a2a490c67731cc94baa082))
* **deps:** update module github.com/anthropics/anthropic-sdk-go to v1.63.1 ([#1163](https://github.com/mogenius/mogenius-operator/issues/1163)) ([76c9fb7](https://github.com/mogenius/mogenius-operator/commit/76c9fb742b8dff3153320fc66a97955eafb8a01e))
* **deps:** update module github.com/ollama/ollama to v0.32.14 ([def83a8](https://github.com/mogenius/mogenius-operator/commit/def83a8fb980ecc36ac4d71fa60bc84fe1e5b114))
* **deps:** update module github.com/ollama/ollama to v0.32.6 ([#1131](https://github.com/mogenius/mogenius-operator/issues/1131)) ([56264f5](https://github.com/mogenius/mogenius-operator/commit/56264f5dadf59862d0aa024ca4bd01e1fbbf27d9))
* **deps:** update module github.com/ollama/ollama to v0.32.9 ([9f9e027](https://github.com/mogenius/mogenius-operator/commit/9f9e0277e7f502a2804291175b966f74b89d4c10))
* **deps:** update module github.com/openai/openai-go/v3 to v3.51.0 ([#1166](https://github.com/mogenius/mogenius-operator/issues/1166)) ([96555f2](https://github.com/mogenius/mogenius-operator/commit/96555f2b209246b939331d2d6f7221d8636e5345))
* **deps:** update module github.com/valkey-io/valkey-go to v1.0.77 ([9962f97](https://github.com/mogenius/mogenius-operator/commit/9962f9765e6989820b5be085a2c376d350314854))
* **deps:** update module helm.sh/helm/v4 to v4.2.4 ([#1162](https://github.com/mogenius/mogenius-operator/issues/1162)) ([329eca6](https://github.com/mogenius/mogenius-operator/commit/329eca6ee1f581f1351900236f3e1e380e5e808e))
* downgrade snoopy 0.4.14 is broken ([0b845d7](https://github.com/mogenius/mogenius-operator/commit/0b845d71bfd47a9ae3339bbb5801fb9a3b1f4507))
* **gitops:** repair flux install path and argo finalizer placement ([12201ce](https://github.com/mogenius/mogenius-operator/commit/12201cebe1d259e65692e6cb3ae31773cc3392e4))
* **gitops:** secret reference for gitops write secrets ([e3d95c9](https://github.com/mogenius/mogenius-operator/commit/e3d95c977b427449ea617d0b300b8c35ca48d327))
* pass parent context instead of Background for api-call ([f3d0b7e](https://github.com/mogenius/mogenius-operator/commit/f3d0b7e7e1cd30f75ccba47f11841d3659063baf))
* pass toolapproval data to ai run ([2fa17fa](https://github.com/mogenius/mogenius-operator/commit/2fa17fa8ee810e55e7c24b76fa25bcd26935fafb))
* **platformConfig:** set revisionHistoryLimit for all argocd apps ([01622a0](https://github.com/mogenius/mogenius-operator/commit/01622a009d4283a4da337015cac58a5efe081f7a))
* preserve dashboardRef and displayName on partial workspace updates ([b1f1232](https://github.com/mogenius/mogenius-operator/commit/b1f123239979b9ecf2b8db1fea36eebadab12ae8))
* remove auditlog aipromptinject because api sets this up ([bbd05af](https://github.com/mogenius/mogenius-operator/commit/bbd05af702b009c32152dd6280b6d4a830256170))
* stream both OS memory usage and kubernetes working set memory ([d39e27a](https://github.com/mogenius/mogenius-operator/commit/d39e27ae49c97f18cc88a818fc4e9d5f9d624ab0))
* **test:** adding harness for e2e tests and fixing valkey issue ([e77cd62](https://github.com/mogenius/mogenius-operator/commit/e77cd620b1eda52be5aff3376818c9ee2f15d937))
* **valkey:** enumerate keys per node and read resource lists from the index ([d98cce5](https://github.com/mogenius/mogenius-operator/commit/d98cce5981b316df73949b02060d2027308841c3))
* **valkey:** use domulti for multiple bacth commands to be valkey cluster ready ([4e9c2d8](https://github.com/mogenius/mogenius-operator/commit/4e9c2d8af7d2e126c45694c52bf196ad886a48b3))

## [2.27.1](https://github.com/mogenius/mogenius-operator/compare/v2.27.0...v2.27.1) (2026-08-06)


### Bug Fixes

* **helm:** stop hardcoding GOMEMLIMIT, it caused a GC death spiral (MOG-4518) ([fc2bac3](https://github.com/mogenius/mogenius-operator/commit/fc2bac3bf97936b1a93335c0b15c39797f6096d4))
* merge develop to main for the last time ([93ec410](https://github.com/mogenius/mogenius-operator/commit/93ec410fde0837b350e0684728127dc414f11631))
* **reconciler:** bound the store-readiness wait instead of requeueing forever ([406f96e](https://github.com/mogenius/mogenius-operator/commit/406f96e276f2fa8bd9e2ef882826233f573d6ea8))


### Performance Improvements

* **store:** read resource lists through the index instead of scanning ([14c2473](https://github.com/mogenius/mogenius-operator/commit/14c24735fbb2a2aad83a4a1fcf5538f4c48790e1))
