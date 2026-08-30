import asyncio

from ghanageo import AsyncGhanaGeo


async def main() -> None:
    async with AsyncGhanaGeo() as client:
        page = await client.search("Kumasi", limit=5)
        for result in page.data:
            print(result.id, result.name)


asyncio.run(main())
