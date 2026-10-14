---@meta

---@alias phase "receive"|"deliver"
---@alias kind "post"|"edit"|"delete"|"typing"
---@alias cap "reply"|"edit"|"delete"|"typing"|"puppet"|"media"

---@param phase phase
---@param fn fun(ev: Event, dst?: Target): false?
function on(phase, fn) end

---@param s string
---@return string
function hash(s) end

---@class Span
---@field kind "text"|"mention"
---@field text string
local Span = {}
---@return string
function Span:display() end

---@class File
---@field name string
---@field mime string
---@field url string
---@field public boolean
---@field key string
local File = {}
---@return boolean
function File:empty() end

---@class Author
---@field id string
---@field name string
---@field avatar File
local Author = {}
---@return string
function Author:display() end

---@class Message
---@field author Author
---@field content Span[]
---@field files File[]
local Message = {}
---@alias part string|Span[]

---@param ... part
function Message:prepend(...) end

---@param ... part
function Message:append(...) end

---@param ... part
function Message:set(...) end

---@param n integer
---@return Span[]
function Message:excerpt(n) end

---@class Event : Message
---@field kind kind
---@field platform string
local Event = {}

---@class Target
---@field account string
---@field channel string
---@field platform string
---@field parent Message?
local Target = {}
---@param name cap
---@return boolean
function Target:can(name) end
