# Changelog

## [0.1.0](https://github.com/f1nniboy/bridgery/compare/v0.0.1...v0.1.0) (2026-10-15)


### Features

* add logo ([8b6d131](https://github.com/f1nniboy/bridgery/commit/8b6d131481b5ae3fbb526d3bf29e9b00de95181f))
* **config:** read secret fields from environment variables ([c154f46](https://github.com/f1nniboy/bridgery/commit/c154f46400edd98995cb504e9249c5d9b334d97a))
* **config:** rename channel modes to send, receive and both ([de9ce73](https://github.com/f1nniboy/bridgery/commit/de9ce730677796f219396c0aa50711d24993f917))
* **core:** add lazy file fetching ([2088fa7](https://github.com/f1nniboy/bridgery/commit/2088fa7abe570bd5afb4c15d47914261c76357e3))
* **ledger:** add sqlite backend ([be59565](https://github.com/f1nniboy/bridgery/commit/be595658d1116f5d4b03f2bd37b9ad8dc0b2d13c))
* **ledger:** forget messages after max age ([be59565](https://github.com/f1nniboy/bridgery/commit/be595658d1116f5d4b03f2bd37b9ad8dc0b2d13c))
* **media:** sniff missing mime types before storing ([2088fa7](https://github.com/f1nniboy/bridgery/commit/2088fa7abe570bd5afb4c15d47914261c76357e3))
* **platforms/discord:** skip attachments over the upload limit ([11407f3](https://github.com/f1nniboy/bridgery/commit/11407f33074776af364c7f7fa3372d1cad776c55))
* **platforms/irc:** fail joins on join error numerics ([442f5ea](https://github.com/f1nniboy/bridgery/commit/442f5ea442a6141f9ced044147d6fe3255222856))
* **platforms/matrix:** add ([84d109b](https://github.com/f1nniboy/bridgery/commit/84d109b2be80b9f8bff7d2789f5b09eb5b0ebbd9))
* **platforms/xmpp:** bridge incoming out-of-band attachments ([11407f3](https://github.com/f1nniboy/bridgery/commit/11407f33074776af364c7f7fa3372d1cad776c55))


### Bug fixes

* **config:** durations without trailing zero units ([e873062](https://github.com/f1nniboy/bridgery/commit/e8730627a675a83c5ccee949a44190b1b3fa2a06))
* exit immediately on a second interrupt signal ([fc4f86c](https://github.com/f1nniboy/bridgery/commit/fc4f86ce5917e92604474d3431e1924fdbfeee3a))
* **gateway:** compare edits after receive hooks ([a578235](https://github.com/f1nniboy/bridgery/commit/a57823597eb04f7051c20131376ec8a11ece9037))
* **gateway:** deliver queued events before closing bridges on shutdown ([fc4f86c](https://github.com/f1nniboy/bridgery/commit/fc4f86ce5917e92604474d3431e1924fdbfeee3a))
* **gateway:** drop remaining queued events after deadline ([fc4f86c](https://github.com/f1nniboy/bridgery/commit/fc4f86ce5917e92604474d3431e1924fdbfeee3a))
* **gateway:** skip deliveries to accounts that are not connected ([fce13a2](https://github.com/f1nniboy/bridgery/commit/fce13a2eda9f04a3f7aafec8cef46f68eabbb186))
* **gateway:** wait for in-flight deliveries before closing or reopening a bridge ([fce13a2](https://github.com/f1nniboy/bridgery/commit/fce13a2eda9f04a3f7aafec8cef46f68eabbb186))
* **media:** detach shared downloads from the first caller, write local files atomically ([11407f3](https://github.com/f1nniboy/bridgery/commit/11407f33074776af364c7f7fa3372d1cad776c55))
* **platforms/irc:** match channel names using the server's casemapping ([a578235](https://github.com/f1nniboy/bridgery/commit/a57823597eb04f7051c20131376ec8a11ece9037))
* **platforms/irc:** reconnect through the router instead of ircevent ([442f5ea](https://github.com/f1nniboy/bridgery/commit/442f5ea442a6141f9ced044147d6fe3255222856))
* **platforms/irc:** strip formatting codes from incoming messages ([58d82a1](https://github.com/f1nniboy/bridgery/commit/58d82a134d3624bf9419b704815b12929e72ef38))
* **platforms/xmpp:** add an extension to re-hosted avatars ([11407f3](https://github.com/f1nniboy/bridgery/commit/11407f33074776af364c7f7fa3372d1cad776c55))
* **platforms/xmpp:** close the tcp connection on handshake failure and on close ([fce13a2](https://github.com/f1nniboy/bridgery/commit/fce13a2eda9f04a3f7aafec8cef46f68eabbb186))
* **platforms/xmpp:** dont log lost connection when the bridge is closed on purpose ([6785695](https://github.com/f1nniboy/bridgery/commit/6785695dea6670ac0e67f9f9391154c55dfe7c26))
* **platforms/xmpp:** handle overlapping reply fallback ranges without panicking ([fce13a2](https://github.com/f1nniboy/bridgery/commit/fce13a2eda9f04a3f7aafec8cef46f68eabbb186))
* **platforms/xmpp:** retry transient stream errors on connect ([442f5ea](https://github.com/f1nniboy/bridgery/commit/442f5ea442a6141f9ced044147d6fe3255222856))
* **platforms/xmpp:** time out stalled presence writes ([a578235](https://github.com/f1nniboy/bridgery/commit/a57823597eb04f7051c20131376ec8a11ece9037))
* stop reconnecting after permanent failures on discord and irc ([a578235](https://github.com/f1nniboy/bridgery/commit/a57823597eb04f7051c20131376ec8a11ece9037))


### Performance

* **ledger:** evict messages without scanning all bindings ([11407f3](https://github.com/f1nniboy/bridgery/commit/11407f33074776af364c7f7fa3372d1cad776c55))


### Documentation

* add chat links to README ([be59565](https://github.com/f1nniboy/bridgery/commit/be595658d1116f5d4b03f2bd37b9ad8dc0b2d13c))
* **hooks:** add limitations section, drop live reload note ([a95a4a8](https://github.com/f1nniboy/bridgery/commit/a95a4a8ff08b2fafe529fe55994651761425004c))
* **platforms:** link specs next to protocol-defined values ([11407f3](https://github.com/f1nniboy/bridgery/commit/11407f33074776af364c7f7fa3372d1cad776c55))


### Refactors

* **core:** scope status reports to a session context ([442f5ea](https://github.com/f1nniboy/bridgery/commit/442f5ea442a6141f9ced044147d6fe3255222856))
* **core:** store messages as tagged json, without file bytes ([be59565](https://github.com/f1nniboy/bridgery/commit/be595658d1116f5d4b03f2bd37b9ad8dc0b2d13c))
* **gateway:** build queues in New, simplify route and channel building with Channel.Address ([fc4f86c](https://github.com/f1nniboy/bridgery/commit/fc4f86ce5917e92604474d3431e1924fdbfeee3a))
* **gateway:** make channel joins request/response ([442f5ea](https://github.com/f1nniboy/bridgery/commit/442f5ea442a6141f9ced044147d6fe3255222856))
* **gateway:** rename Gateway to Router, split into inbound, outbound and topology ([fc4f86c](https://github.com/f1nniboy/bridgery/commit/fc4f86ce5917e92604474d3431e1924fdbfeee3a))
* **ledger:** pass context and return errors for every call ([be59565](https://github.com/f1nniboy/bridgery/commit/be595658d1116f5d4b03f2bd37b9ad8dc0b2d13c))
* **platforms/discord:** move message parsing into content.go ([80cf57d](https://github.com/f1nniboy/bridgery/commit/80cf57d72e77df5357da99515c4e13d05d34aeed))
* **platforms/xmpp:** await joins and message echos ([442f5ea](https://github.com/f1nniboy/bridgery/commit/442f5ea442a6141f9ced044147d6fe3255222856))
