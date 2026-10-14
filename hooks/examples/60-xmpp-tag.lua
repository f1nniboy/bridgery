--[[
tags bridged authors with their origin platform on xmpp, so puppets stay
distinct from native occupants (e.g. "foo" becomes "foo [discord]")
--]]

on("deliver", function(ev, dst)
	if dst.platform == "xmpp" and dst:can("puppet") then
		ev.author.name = ev.author:display() .. " [" .. ev.platform .. "]"
	end
end)
