-- Sample Lua plugin (Lua 5.1, gopher-lua).
function apply_filter(ctx)
  ctx.profileName = ctx.profileName .. "_lua"
  if ctx.textures ~= nil and ctx.textures["SKIN"] ~= nil and ctx.textures["SKIN"]["url"] ~= nil then
    ctx.textures["SKIN"]["url"] = string.gsub(ctx.textures["SKIN"]["url"], "example%.com", "cdn.example.com")
  end
  return ctx
end
