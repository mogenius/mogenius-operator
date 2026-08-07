# Changelog

## [2.28.0](https://github.com/mogenius/mogenius-operator/compare/v2.27.1...v2.28.0) (2026-08-07)


### Features

* **gitops:** flux commands, workspace support, helm tagging and ai tools ([08159a1](https://github.com/mogenius/mogenius-operator/commit/08159a1d22baa3c488649aeb1f46f89aeb657aeb))


### Bug Fixes

* allow adding typed config values ([39e288a](https://github.com/mogenius/mogenius-operator/commit/39e288aafe77ff69afe6ec97fa52efc4f50e1ab5))
* **cluster:** report node Ready condition in node stats ([00dd06f](https://github.com/mogenius/mogenius-operator/commit/00dd06f8288644d833608466c6ba1ca63337d7b4))
* delete not needed certmanager package reference ([d419448](https://github.com/mogenius/mogenius-operator/commit/d419448b60f6f5b6e120cb25d7c8a5584e5244eb))
* **deps:** update module github.com/ollama/ollama to v0.32.6 ([#1131](https://github.com/mogenius/mogenius-operator/issues/1131)) ([56264f5](https://github.com/mogenius/mogenius-operator/commit/56264f5dadf59862d0aa024ca4bd01e1fbbf27d9))
* **gitops:** repair flux install path and argo finalizer placement ([12201ce](https://github.com/mogenius/mogenius-operator/commit/12201cebe1d259e65692e6cb3ae31773cc3392e4))
* **platformConfig:** set revisionHistoryLimit for all argocd apps ([01622a0](https://github.com/mogenius/mogenius-operator/commit/01622a009d4283a4da337015cac58a5efe081f7a))

## [2.27.1](https://github.com/mogenius/mogenius-operator/compare/v2.27.0...v2.27.1) (2026-08-06)


### Bug Fixes

* **helm:** stop hardcoding GOMEMLIMIT, it caused a GC death spiral (MOG-4518) ([fc2bac3](https://github.com/mogenius/mogenius-operator/commit/fc2bac3bf97936b1a93335c0b15c39797f6096d4))
* merge develop to main for the last time ([93ec410](https://github.com/mogenius/mogenius-operator/commit/93ec410fde0837b350e0684728127dc414f11631))
* **reconciler:** bound the store-readiness wait instead of requeueing forever ([406f96e](https://github.com/mogenius/mogenius-operator/commit/406f96e276f2fa8bd9e2ef882826233f573d6ea8))


### Performance Improvements

* **store:** read resource lists through the index instead of scanning ([14c2473](https://github.com/mogenius/mogenius-operator/commit/14c24735fbb2a2aad83a4a1fcf5538f4c48790e1))
