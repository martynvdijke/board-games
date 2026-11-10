## [1.11.3](https://github.com/martynvdijke/board-games/compare/v1.11.2...v1.11.3) (2026-11-10)

## [1.11.2](https://github.com/martynvdijke/board-games/compare/v1.11.1...v1.11.2) (2026-11-10)


### Bug Fixes

* **renovate:** disable platform automerge so Renovate merges green PRs itself ([ba9ec8c](https://github.com/martynvdijke/board-games/commit/ba9ec8cb1aee8020b347f47501b6189abb03c85d))

## [1.11.1](https://github.com/martynvdijke/board-games/compare/v1.11.0...v1.11.1) (2026-11-10)


### Bug Fixes

* **deps:** update all non-major dependencies ([#14](https://github.com/martynvdijke/board-games/issues/14)) ([463987d](https://github.com/martynvdijke/board-games/commit/463987d8db9aef7d83475f68f46561b9e5a3423a))

# [1.11.0](https://github.com/martynvdijke/board-games/compare/v1.10.0...v1.11.0) (2026-11-10)


### Features

* **collection:** add acquisition value insights ([626af69](https://github.com/martynvdijke/board-games/commit/626af69baae74502d8f9422d55aec55e01e67e2a))

# [1.10.0](https://github.com/martynvdijke/board-games/compare/v1.9.0...v1.10.0) (2026-11-08)


### Bug Fixes

* **media:** fall back to the full cover and stop caching placeholders ([852dbec](https://github.com/martynvdijke/board-games/commit/852dbece9123a40d399df7dbd60ea23f1ebff310))


### Features

* **notify:** add optional Telegram notifications ([3e8811e](https://github.com/martynvdijke/board-games/commit/3e8811e418ccbafb9ea528858558b00a8c353c66))
* **plays:** add a tick-list of past players to the play form ([1287bf0](https://github.com/martynvdijke/board-games/commit/1287bf04e14272e2980ddbdfd5489ca87104a66a))

# [1.9.0](https://github.com/martynvdijke/board-games/compare/v1.8.0...v1.9.0) (2026-11-08)


### Features

* **mobile:** fetch OpenTelemetry endpoint from API at startup ([823a53d](https://github.com/martynvdijke/board-games/commit/823a53d09998ba4e0f3470e2fa233c783e7d7d74))

# [1.8.0](https://github.com/martynvdijke/board-games/compare/v1.7.0...v1.8.0) (2026-11-05)


### Features

* **privacy:** add Play review demo and EU privacy controls ([a0cc0ab](https://github.com/martynvdijke/board-games/commit/a0cc0ab1201c64d9e0478aaf71130776664ac875))

# [1.7.0](https://github.com/martynvdijke/board-games/compare/v1.6.4...v1.7.0) (2026-11-05)


### Features

* **games:** add private per-game setup guide with photos ([253a204](https://github.com/martynvdijke/board-games/commit/253a204be3f774280fae0a5e9c2edd0d83cfa023))

## [1.6.4](https://github.com/martynvdijke/board-games/compare/v1.6.3...v1.6.4) (2026-11-05)


### Bug Fixes

* **auth:** guarantee an instance always has an admin ([1eed535](https://github.com/martynvdijke/board-games/commit/1eed535ba22e350741488c2d49ee18bee21fbc2c))

## [1.6.3](https://github.com/martynvdijke/board-games/compare/v1.6.2...v1.6.3) (2026-11-05)


### Bug Fixes

* **security:** encrypt BGG API token at rest ([8a831a8](https://github.com/martynvdijke/board-games/commit/8a831a849a7001777312980733dd2434cd1cca99))

## [1.6.2](https://github.com/martynvdijke/board-games/compare/v1.6.1...v1.6.2) (2026-11-04)

## [1.6.1](https://github.com/martynvdijke/board-games/compare/v1.6.0...v1.6.1) (2026-11-04)


### Bug Fixes

* **android:** resolve usesCleartextTraffic manifest merger conflict ([26013af](https://github.com/martynvdijke/board-games/commit/26013af13c086d91bb951aa1c6b621bbcbd04759))

# [1.6.0](https://github.com/martynvdijke/board-games/compare/v1.5.0...v1.6.0) (2026-11-04)


### Features

* **bgg:** support application API tokens ([12967d1](https://github.com/martynvdijke/board-games/commit/12967d194bd95d3a5c6a279caa881c9c318f9bc1))

# [1.5.0](https://github.com/martynvdijke/board-games/compare/v1.4.0...v1.5.0) (2026-11-03)


### Features

* **auth:** read OIDC client secret from file ([dd99756](https://github.com/martynvdijke/board-games/commit/dd99756d8539863e9f0d7fa5323a4dd5a53aef17))

# [1.4.0](https://github.com/martynvdijke/board-games/compare/v1.3.4...v1.4.0) (2026-11-03)


### Bug Fixes

* **deploy:** listen on port 6270 by default ([8bca4b6](https://github.com/martynvdijke/board-games/commit/8bca4b64f93968dedd49a67ff2eac8550d329470))


### Features

* **legal:** GDPR/Play compliance for board-games ([15c676e](https://github.com/martynvdijke/board-games/commit/15c676ef371d639aca35cbc068cf1012b1b5a892))

## [1.3.4](https://github.com/martynvdijke/board-games/compare/v1.3.3...v1.3.4) (2026-11-01)


### Bug Fixes

* **ci:** skip Play Store publish when credentials are missing ([d8edf67](https://github.com/martynvdijke/board-games/commit/d8edf67bada4e93bb66f0f4eb4d6773ce195bd0a))

## [1.3.3](https://github.com/martynvdijke/board-games/compare/v1.3.2...v1.3.3) (2026-10-31)


### Bug Fixes

* **mobile:** guard theme provider state updates after disposal ([#12](https://github.com/martynvdijke/board-games/issues/12)) ([337f6c0](https://github.com/martynvdijke/board-games/commit/337f6c056ceebb1bc1156f22ccf1b95babe7b9ff))

## [1.3.2](https://github.com/martynvdijke/board-games/compare/v1.3.1...v1.3.2) (2026-10-31)


### Bug Fixes

* **ci:** point android e2e at root .fvmrc ([9804a29](https://github.com/martynvdijke/board-games/commit/9804a29a2ffe7d09cfe8f61f022fc9b8d7d0f056))
* **ci:** point android e2e flutter setup at root fvmrc ([9cf4294](https://github.com/martynvdijke/board-games/commit/9cf429425c38cad5e2c5d30c48bdf856e011a617))
* **ci:** point play store flutter setup at root fvmrc ([b0d731b](https://github.com/martynvdijke/board-games/commit/b0d731b03cec2d59d433f41de6aea6f3d926680c))
* **ci:** use root .fvmrc in Play Store workflow ([728c33c](https://github.com/martynvdijke/board-games/commit/728c33c51726d90b9e5fbeace4156ae26ddaa52b))
* **deps:** update all non-major dependencies ([3f95c6f](https://github.com/martynvdijke/board-games/commit/3f95c6f9f7de63eb964797e2481cd283869a7e35))

## [1.3.1](https://github.com/martynvdijke/board-games/compare/v1.3.0...v1.3.1) (2026-10-31)

# [1.3.0](https://github.com/martynvdijke/board-games/compare/v1.2.1...v1.3.0) (2026-10-31)


### Features

* **mobile:** default to board-games.vandijke.xyz and offer self-hosted option ([7c50a61](https://github.com/martynvdijke/board-games/commit/7c50a610a2f8422c1cdf6ac5bfd39bae988b25a6))

## [1.2.1](https://github.com/martynvdijke/board-games/compare/v1.2.0...v1.2.1) (2026-10-27)

# [1.2.0](https://github.com/martynvdijke/board-games/compare/v1.1.2...v1.2.0) (2026-10-27)


### Features

* **release:** publish android bundles to google play and add emulator e2e ([c54aca4](https://github.com/martynvdijke/board-games/commit/c54aca46523448e0a76909d8ccc92aa644250fbf))

## [1.1.2](https://github.com/martynvdijke/board-games/compare/v1.1.1...v1.1.2) (2026-10-26)

## [1.1.1](https://github.com/martynvdijke/board-games/compare/v1.1.0...v1.1.1) (2026-10-26)


### Bug Fixes

* **deps:** update dependency cupertino_icons to v2 ([cdf030c](https://github.com/martynvdijke/board-games/commit/cdf030c81c1b9b5e953ed88e36554af87539003f))

# [1.1.0](https://github.com/martynvdijke/board-games/compare/v1.0.2...v1.1.0) (2026-10-25)


### Features

* add Android companion app ([9e7bfda](https://github.com/martynvdijke/board-games/commit/9e7bfdaf472df8e3e290c9da2fe60989aba32c9a))

## [1.0.2](https://github.com/martynvdijke/board-games/compare/v1.0.1...v1.0.2) (2026-10-24)

## [1.0.1](https://github.com/martynvdijke/board-games/compare/v1.0.0...v1.0.1) (2026-10-24)

# 1.0.0 (2026-10-23)


### Features

* initial release of The Shelf ([7a7aee8](https://github.com/martynvdijke/board-games/commit/7a7aee8ee19135d5f550c2bf64ee0976a29170a0))
