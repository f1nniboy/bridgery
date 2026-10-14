--[[
quotes the replied to message inline for platforms that don't natively support
replies
--]]

on("deliver", function(ev, dst)
	if not (ev.kind == "post" or ev.kind == "edit") or dst.parent == nil or dst:can("reply") then
		return
	end

	local who = dst.parent.author:display()
	local excerpt = dst.parent:excerpt(50)

	-- add all platforms that should use the pretty format here
	if dst.platform == "discord" then
		ev:prepend("> ↪ @" .. who .. "\n> ", excerpt, "\n")
	else
		ev:append(" (in reply to @" .. who .. ": ", excerpt, ")")
	end
end)
