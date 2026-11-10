// lazytodo.ts — let the agent drive the lazytodo TUI running in this directory.
//
// The TUI keeps its list in `.lazytodo.json` and re-reads the file whenever its
// mtime changes, so this extension only has to do a safe read-modify-write:
// whatever it writes shows up on screen within half a second.
//
// Mutations go through withFileMutationQueue so they take the same per-file lock
// as the built-in edit/write tools. Without it, two parallel tool calls could
// both read the old list, compute different results, and the last write would
// silently win.
//
// The file format is the TUI's own: { nextId, todos: [{ id, title, detail?,
// status }] }, status being "todo" | "doing" | "done". Older files that only
// carried a `done` bool are read correctly.

import { withFileMutationQueue, type ExtensionAPI } from "@earendil-works/pi-coding-agent"
import { StringEnum } from "@earendil-works/pi-ai"
import { Type } from "typebox"
import { readFile, rename, writeFile } from "node:fs/promises"
import { join } from "node:path"

const FILE_NAME = ".lazytodo.json"
const STATUSES = ["todo", "doing", "done"] as const

type Status = (typeof STATUSES)[number]

type Todo = {
  id: number
  title: string
  detail?: string
  status: Status
}

type Store = {
  nextId: number
  todos: Todo[]
}

const MARKERS: Record<Status, string> = { todo: "○", doing: "◐", done: "✓" }

async function loadStore(path: string): Promise<Store> {
  let raw: string
  try {
    raw = await readFile(path, "utf8")
  } catch (error) {
    if ((error as NodeJS.ErrnoException).code === "ENOENT") {
      return { nextId: 1, todos: [] }
    }
    throw error
  }

  const parsed = JSON.parse(raw) as { nextId?: number; todos?: unknown[] }
  const todos: Todo[] = (parsed.todos ?? []).map((entry) => {
    const todo = entry as Partial<Todo> & { done?: boolean }
    const status = STATUSES.includes(todo.status as Status)
      ? (todo.status as Status)
      : todo.done
        ? "done"
        : "todo"

    const normalised: Todo = {
      id: todo.id ?? 0,
      title: todo.title ?? "",
      status,
    }
    if (todo.detail) normalised.detail = todo.detail
    return normalised
  })

  const highest = todos.reduce((max, todo) => Math.max(max, todo.id), 0)
  return { nextId: parsed.nextId ?? highest + 1, todos }
}

async function saveStore(path: string, store: Store): Promise<void> {
  // The same atomic write the TUI does: a sibling file, then rename over the
  // target, so the TUI never reads a half-written list.
  const temporary = `${path}.agent.tmp`
  await writeFile(temporary, `${JSON.stringify(store, null, 2)}\n`, "utf8")
  await rename(temporary, path)
}

function render(todos: Todo[]): string {
  if (todos.length === 0) return "(no todos)"
  return todos.map((todo) => `${MARKERS[todo.status]} #${todo.id} ${todo.title}`).join("\n")
}

export default async function (pi: ExtensionAPI) {
  pi.registerTool({
    name: "lazytodo",
    label: "Lazytodo",
    description:
      "Read and change the todos of the lazytodo TUI for this directory. The list lives in .lazytodo.json and a running TUI reloads it automatically.",
    promptSnippet: "List, add, update or remove todos in this directory's lazytodo list",
    promptGuidelines: [
      "Use lazytodo when the user wants work tracked in the lazytodo TUI; do not edit .lazytodo.json by hand.",
    ],
    parameters: Type.Object({
      action: StringEnum(["list", "add", "set", "remove"] as const),
      title: Type.Optional(Type.String({ description: "Todo title (action=add)" })),
      detail: Type.Optional(
        Type.String({ description: "Multi-line description (action=add)" })
      ),
      id: Type.Optional(Type.Number({ description: "Todo id (action=set/remove)" })),
      status: Type.Optional(StringEnum(STATUSES)),
      filter: Type.Optional(StringEnum(STATUSES)),
    }),

    async execute(_toolCallId, params, signal, _onUpdate, ctx) {
      if (signal?.aborted) {
        return { content: [{ type: "text", text: "Cancelled" }], details: {} }
      }

      const path = join(ctx.cwd, FILE_NAME)

      // Every answer ends with the resulting list, so the model sees the state it
      // just produced without spending another call on `list`.
      const reply = async (message: string) => {
        const store = await loadStore(path)
        return {
          content: [{ type: "text", text: `${message}\n\n${render(store.todos)}` }],
          details: { path, todos: store.todos },
        }
      }

      const mutate = (change: (store: Store) => string) =>
        withFileMutationQueue(path, async () => {
          const store = await loadStore(path)
          const message = change(store)
          await saveStore(path, store)
          return reply(message)
        })

      switch (params.action) {
        case "list": {
          const store = await loadStore(path)
          const todos = params.filter
            ? store.todos.filter((todo) => todo.status === params.filter)
            : store.todos
          return {
            content: [{ type: "text", text: render(todos) }],
            details: { path, todos: store.todos },
          }
        }

        case "add": {
          const title = params.title?.trim()
          if (!title) {
            return reply("error: title is required for action=add")
          }

          return mutate((store) => {
            const id = store.nextId
            store.nextId += 1

            const todo: Todo = { id, title, status: "todo" }
            if (params.detail) todo.detail = params.detail
            store.todos.push(todo)

            return `Added #${id}: ${title}`
          })
        }

        case "set": {
          if (typeof params.id !== "number") {
            return reply("error: id is required for action=set")
          }
          if (!params.status) {
            return reply("error: status is required for action=set")
          }

          return mutate((store) => {
            const todo = store.todos.find((candidate) => candidate.id === params.id)
            if (!todo) return `error: no todo with id ${params.id}`

            const previous = todo.status
            todo.status = params.status as Status
            return `#${todo.id} ${previous} -> ${todo.status}: ${todo.title}`
          })
        }

        case "remove": {
          if (typeof params.id !== "number") {
            return reply("error: id is required for action=remove")
          }

          return mutate((store) => {
            const index = store.todos.findIndex((candidate) => candidate.id === params.id)
            if (index === -1) return `error: no todo with id ${params.id}`

            const [removed] = store.todos.splice(index, 1)
            return `Removed #${removed.id}: ${removed.title}`
          })
        }

        default:
          return reply(`error: unknown action ${String(params.action)}`)
      }
    },
  })
}
