# Changelog

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
