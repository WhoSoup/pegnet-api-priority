# PegNet API Priority Determiner

A small tool that attempts to determine a data source priority order that should result in being able to successfully mine in PegNet.

# Setup

The app takes the [default PegNet reference miner configuration](https://github.com/pegnet/pegnet/blob/master/config/defaultconfig.ini), located at the default location: `%HOME%/.pegnet/defaultconfig.ini`. 

* `[Miner].FactomdLocation` (NO trailing /v2) sets the location of the factomd endpoint. You can use the open node with this.
* `[OracleDataSources]` configures which sources should be active. Data sources that require an API key need to be configured in the `[Oracle]` section.

The app will only pull data from sources that are configured. For the optimal results, all sources should be enabled. In practice, just enabling all the free sources should suffice.

There are a few flags:

* `server`: if this flag is present, the debug server will be launched. Default location is `http://localhost:8080/`.
* `port`: specify the port of the debug server (default 8080)
* `debug-save`: if you specify a file, the data gathered will be dumped into there at the beginning of every block
* `debug-read`: if you specify a file containing a file dump from "debug-save", the app will try to load that data. in this mode, the app will only display the contents of the file but not gather new data

# How it works

The app waits for the same events that miners do. At minute 1 of a new block, it pulls the prices. In the next block, it downloads the OPRs that were submitted and compares them to the prices from the last block. The APIs are then sorted by their "win percentage", where a "win" is defined as "the api's price for that asset is closest to the opr". Additionally, a "band check" is performed that tests whether your current configuration is within 1% of the other OPRs.

Example:

OPR from miner "WhoSoup":
* PEG: 0.1
* BTC: 9000
* FCT: 2

Source A:
* PEG: 0.11
* BTC: 9000
* FCT: 2.1

Source B:
* PEG: 0.1
* BTC: 8999
* FCT: 2

For PEG, Source B is closer. For BTC, A is closer. For FCT, B is closer. This results in B having a "win percentage" of 2/3, and A has 1/3. The resulting priority order would be `Source A=0, Source B=1`. 

# Running 

In addition to the console output, V2 adds a graphical analysis of miner submissions and API results. You can view the blocks at `http://localhost:8080/`. Please be aware that not all blocks have complete information. The first block on the list will not yet have the OPRs from mining, and the last block on the list will not have API data.

**Expect to be running this for around 15 minutes before the first results show.**

The app prints out a lot of status messages, starting with the currently configured data sources listed in order of their priority. Every minute, the app prints out how many minutes are left until the next check. The first check will grab the data from all datasources. In the next block, those price points are compared. 

Example outout:
> 2020/04/02 12:14:05 12 DataSources Loaded: FixedUSD (0), PegnetMarketCap (1), OpenExchangeRates (2), FreeForexAPI (3), AlternativeMe (4), CoinGecko (5), 1Forge (6), CoinCap (7), CoinMarketCap (8), APILayer (9), Factoshiio (10), Kitco (11)
> 
> 2020/04/02 12:14:06 Initializing App at height 238767, minute 8, waiting for next block...
> 
> 2020/04/02 12:14:06 3 minutes until next entry check
> 
> 2020/04/02 12:15:00 2 minutes until next entry check
> 
> 2020/04/02 12:16:01 1 minutes until next entry check
> 
> 2020/04/02 12:17:05 Querying Datasources to use for block 238769
> 
> 2020/04/02 12:17:10 Datasources fetched in 5.0798786s
> 
> 2020/04/02 12:17:10 Don't have last block's prices to compare entries to, need to wait for next block
> 
> 2020/04/02 12:18:01 9 minutes until next entry check
> 
> 2020/04/02 12:19:00 8 minutes until next entry check
> 
> 2020/04/02 12:20:03 7 minutes until next entry check
> 
> 2020/04/02 12:21:00 6 minutes until next entry check
> 
> 2020/04/02 12:22:01 5 minutes until next entry check
> 
> 2020/04/02 12:23:04 4 minutes until next entry check
> 
> 2020/04/02 12:24:00 3 minutes until next entry check
> 
> 2020/04/02 12:25:00 2 minutes until next entry check
> 
> 2020/04/02 12:26:00 1 minutes until next entry check
> 
> 2020/04/02 12:26:01 1 minutes until next entry check
> 
> 2020/04/02 12:27:01 Querying Datasources to use for block 238770
> 
> 2020/04/02 12:27:06 Datasources fetched in 5.2363002s
> 
> 2020/04/02 12:27:06 Downloading entries...
> 
> 2020/04/02 12:27:20 Entries downloaded in 14.1919697s
> 
> =============== Report for Height 238768 ===============
> 
> OPR [miner = FlyingDutchman]
> 
>   Used: FixedUSD=0 PegnetMarketCap=1 FreeForexAPI=2 AlternativeMe=3 CoinGecko=4
> 
> Unused: OpenExchangeRates=5 1Forge=6 CoinCap=7 CoinMarketCap=8 APILayer=9 Factoshiio=10 Kitco=11
> 
> 
> 
> OPR [miner = f2pool]
> 
>   Used: FixedUSD=0 PegnetMarketCap=1 FreeForexAPI=2 AlternativeMe=3 CoinCap=4
> 
> Unused: OpenExchangeRates=5 CoinGecko=6 1Forge=7 CoinMarketCap=8 APILayer=9 Factoshiio=10 Kitco=11
> 
> 
> 
> OPR [miner = orax]
> 
>   Used: FixedUSD=0 PegnetMarketCap=1 FreeForexAPI=2 AlternativeMe=3 CoinCap=4
> 
> Unused: OpenExchangeRates=5 CoinGecko=6 1Forge=7 CoinMarketCap=8 APILayer=9 Factoshiio=10 Kitco=11
> 
> 
> 
> OPR [miner = Schr0dinger]
> 
>   Used: FixedUSD=0 PegnetMarketCap=1 FreeForexAPI=2 AlternativeMe=3 CoinCap=4
> 
> Unused: OpenExchangeRates=5 CoinGecko=6 1Forge=7 CoinMarketCap=8 APILayer=9 Factoshiio=10 Kitco=11
> 
> 
> 
> OPR [miner = bOrax]
> 
>   Used: FixedUSD=0 PegnetMarketCap=1 FreeForexAPI=2 AlternativeMe=3 CoinCap=4
> 
> Unused: OpenExchangeRates=5 CoinGecko=6 1Forge=7 CoinMarketCap=8 APILayer=9 Factoshiio=10 Kitco=11
> 
> ========================================================
> 
> Total
> 
>   Used: FixedUSD=0 PegnetMarketCap=1 FreeForexAPI=2 AlternativeMe=3 CoinCap=4 CoinGecko=5
> 
> Unused: OpenExchangeRates=6 1Forge=7 CoinMarketCap=8 APILayer=9 Factoshiio=10 Kitco=11
> 
> ========================================================
> 
> 2020/04/02 12:27:20 Current priority order is within 1% of 114/114 OPRs (100.00%)

To exit, press CTRL+C.
