# gdal-reverse-proxy

A HTTP reverse-proxy which provides a distributed caching layer for GDAL-based applications with remote data access, ideal for cloud optimized data formats (ex. COG) which are commonly accessed via HTTP range requests.  This project uses [groupcache](https://github.com/golang/groupcache) for distributed cache filling across potentially multiple peers.  This simplifies the deployment of the cache and lets us support large cache sizes by scaling peers horizontally across potentially multiple underlying nodes.

<p align="center">
    <img src="./assets/concept-diagram.png" width="500">
</p>

## Usage
Deploy the proxy with three peers and an NGINX server (`:8000`) load balancing across the peers.
```
docker compose up --build
```

Send a `gdalinfo` call to a publicly available S2 image through the proxy using `/vsicurl`.  The second time running this command should be much faster than the first because the initial HTTP range request to fetch the COG header is cached by the proxy.
```
CPL_CURL_VERBOSE=YES \
GDAL_DISABLE_READDIR_ON_OPEN=TRUE \
CPL_VSIL_CURL_ALLOWED_EXTENSIONS=.tif \
GDAL_HTTP_PROXY=localhost:8000 \
gdalinfo /vsicurl/http://sentinel-cogs.s3.amazonaws.com/sentinel-s2-l2a-cogs/57/C/VK/2019/11/S2B_57CVK_20191122_0_L2A/B04.tif
```

For data in S3 that requires authentication (e.g. private buckets or public "requestor pays" buckets) use `/vsis3`:

``` 
CPL_CURL_VERBOSE=YES \
GDAL_DISABLE_READDIR_ON_OPEN=TRUE \
CPL_VSIL_CURL_ALLOWED_EXTENSIONS=.tif \
AWS_HTTPS=no \
AWS_REQUEST_PAYER=requester \
GDAL_HTTP_PROXY=localhost:8000 \
gdalinfo /vsis3/usgs-landsat/collection02/level-2/standard/oli-tirs/2025/086/075/LC09_L2SR_086075_20250501_20250502_02_T2/LC09_L2SR_086075_20250501_20250502_02_T2_SR_B2.TIF
```

## Limitations
- `/vsicurl` and `/viss3` only, `/vsigs` and `/vsiaz` not yet supported.
- Does not honor or forward `X-Forwarded-*` headers.
- No HTTPS on inbound requests.