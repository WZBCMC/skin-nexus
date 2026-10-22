// Sample JS plugin (goja, ES5.1-safe subset — no optional chaining/nullish
// coalescing/async, per the goja subset-management tax identified in review).
function applyFilter(ctx) {
  ctx.profileName = ctx.profileName + "_js";
  if (ctx.textures && ctx.textures.SKIN && ctx.textures.SKIN.url) {
    ctx.textures.SKIN.url = String(ctx.textures.SKIN.url).split("example.com").join("cdn.example.com");
  }
  return ctx;
}
