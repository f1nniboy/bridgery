--[[
adds the author name to the start of the message for platforms that don't
natively support puppets
--]]

on("deliver", function(ev, dst)
	if (ev.kind == "post" or ev.kind == "edit") and not dst:can("puppet") then
		ev:prepend("<" .. ev.author:display() .. "> ")
	end
end)
