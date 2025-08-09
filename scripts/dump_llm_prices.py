#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = [
#     "llm-price-scraper>=1.0.3"
# ]
# ///

from __future__ import annotations

import csv
import sys
from pathlib import Path
from typing import Sequence

from llm_price_scraper.scrapers import LlmPricingScraper, DataSources  # type: ignore

# --------------------------------------------------------------------------- #
# Helpers
# --------------------------------------------------------------------------- #
FIELDS: Sequence[str] = (
    "model",
    "provider",
    "input_tokens_price",
    "output_tokens_price",
    "context",
    "source",
    "updated",
)


def scrape_prices() -> list:
    """
    Return a list of pricing objects obtained from the default data source
    (DocsBot) via llm_price_scraper.  You can switch source by passing a value
    from `DataSources`, e.g.:
        LlmPricingScraper.scrape(DataSources.HUGGINGFACE)
    """
    return LlmPricingScraper.scrape()  # returns list[LLMModelPricing]


def dump_to_csv(rows: list, path: Path) -> None:
    """Write `rows` (objects with public attrs named in FIELDS) to `path`."""
    with path.open("w", newline="", encoding="utf-8") as fp:
        writer = csv.writer(fp)
        writer.writerow(FIELDS)  # header
        for r in rows:
            writer.writerow([getattr(r, f) for f in FIELDS])


# --------------------------------------------------------------------------- #
# Main
# --------------------------------------------------------------------------- #
def main() -> None:
    outfile = Path(sys.argv[1]) if len(sys.argv) > 1 else Path("openai_llm_prices.csv")
    print(f"⏳  Scraping price data …")
    data = scrape_prices()
    print(f"✅  {len(data)} rows scraped")
    dump_to_csv(data, outfile)
    print(f"💾  CSV written to {outfile.resolve()}")


if __name__ == "__main__":
    main()
