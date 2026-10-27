// texture-cdn: rewrite skin texture URLs to a CDN mirror.
// Offline-safe example: no network calls from the plugin itself; the rewrite
// only changes the URL string embedded in responses.
var CDN_BASE = "https://cdn.example.com";

skin.hooks.onTextureURLFilter(function (e) {
  if (e.type !== "skin") {
    return undefined; // leave capes on the host
  }
  return CDN_BASE + "/textures/" + e.hash;
});
