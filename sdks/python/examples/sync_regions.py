from ghanageo import GhanaGeo

with GhanaGeo() as client:
    page = client.regions(limit=5)
    for region in page.data:
        print(region.id, region.name)
