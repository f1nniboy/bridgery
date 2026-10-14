--[[
uses a fallback avatar for authors without one on platforms that support
puppets
--]]

-- play around with styles at https://www.dicebear.com/playground/
local style = "identicon"

on("deliver", function(ev, dst)
	if ev.author.avatar:empty() and dst:can("puppet") then
		ev.author.avatar.url = "https://api.dicebear.com/10.x/" .. style .. "/png?seed=" .. hash(ev.author:display())
		ev.author.avatar.public = true
	end
end)
