--[[
adds an "(edited)" suffix to edited messages on platforms that don't natively
support edits
--]]

on("deliver", function(ev, dst)
	if ev.kind == "edit" and not dst:can("edit") then
		ev.kind = "post"
		ev:append(" (edited)")
	end
end)
