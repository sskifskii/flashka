import {
  DragEvent,
  FormEvent,
  useEffect,
  useId,
  useMemo,
  useState,
} from "react";
import { api, FolderNode, RequestError } from "../../shared/api";
import { normalizeFolderName, validateFolderName } from "./validation";

export type FlatFolder = {
  id: string;
  name: string;
  label: string;
  depth: number;
};

export function flattenFolders(root: FolderNode): FlatFolder[] {
  const result: FlatFolder[] = [];
  const visit = (node: FolderNode, depth: number) => {
    result.push({
      id: node.id,
      name: node.name,
      label: `${"— ".repeat(depth)}${node.name}`,
      depth,
    });
    node.children.forEach((child) => visit(child, depth + 1));
  };
  visit(root, 0);
  return result;
}

export function FolderSelect({
  root,
  value,
  onChange,
}: {
  root: FolderNode;
  value: string;
  onChange: (id: string) => void;
}) {
  return (
    <select
      id="folder"
      value={value}
      onChange={(e) => onChange(e.target.value)}
    >
      {flattenFolders(root).map((folder) => (
        <option value={folder.id} key={folder.id}>
          {folder.label}
        </option>
      ))}
    </select>
  );
}

function TreeNode({
  node,
  selectedId,
  onSelect,
  draggedId,
  dropTargetId,
  movePending,
  canDropOn,
  onDragTarget,
  onDragStart,
  onDragEnd,
  onDrop,
  showChildren = true,
}: {
  node: FolderNode;
  selectedId: string;
  onSelect: (node: FolderNode) => void;
  draggedId: string | null;
  dropTargetId: string | null;
  movePending: boolean;
  canDropOn: (targetId: string) => boolean;
  onDragTarget: (targetId: string | null) => void;
  onDragStart: (event: DragEvent<HTMLDivElement>, node: FolderNode) => void;
  onDragEnd: () => void;
  onDrop: (sourceId: string, targetId: string) => Promise<void>;
  showChildren?: boolean;
}) {
  const [collapsed, setCollapsed] = useState(false);
  const childrenId = useId();
  const hasChildren = showChildren && node.children.length > 0;

  return (
    <li>
      <div
        className={`folder-node ${selectedId === node.id ? "selected" : ""} ${draggedId === node.id ? "dragging" : ""} ${dropTargetId === node.id ? "drop-target" : ""}`}
        draggable={!node.isInbox && !movePending}
        onDragStart={(event) => onDragStart(event, node)}
        onDragEnd={onDragEnd}
        onDragEnter={(event) => {
          event.stopPropagation();
          if (canDropOn(node.id)) {
            setCollapsed(false);
            onDragTarget(node.id);
          } else {
            onDragTarget(null);
          }
        }}
        onDragOver={(event) => {
          event.stopPropagation();
          if (!canDropOn(node.id)) return;
          event.preventDefault();
          event.dataTransfer.dropEffect = "move";
          onDragTarget(node.id);
        }}
        onDragLeave={(event) => {
          event.stopPropagation();
          const next = event.relatedTarget;
          if (!(next instanceof Node) || !event.currentTarget.contains(next)) {
            onDragTarget(null);
          }
        }}
        onDrop={(event) => {
          event.preventDefault();
          event.stopPropagation();
          const sourceId =
            event.dataTransfer.getData("application/x-flashka-folder-id") ||
            draggedId;
          if (sourceId && canDropOn(node.id)) {
            void onDrop(sourceId, node.id);
          }
        }}
      >
        {hasChildren ? (
          <button
            type="button"
            className="folder-node-toggle"
            aria-label={`${collapsed ? "Развернуть" : "Свернуть"} папку ${node.name}`}
            aria-expanded={!collapsed}
            aria-controls={childrenId}
            onClick={() => setCollapsed((value) => !value)}
          >
            <span
              className={`tree-chevron ${collapsed ? "" : "expanded"}`}
              aria-hidden="true"
            >
              ›
            </span>
            <span className="folder-icon" aria-hidden="true" />
          </button>
        ) : (
          <span className="folder-node-decoration" aria-hidden="true">
            <span className="tree-chevron" />
            <span className="folder-icon" />
          </span>
        )}
        <button
          type="button"
          className="folder-node-select"
          aria-current={selectedId === node.id ? "page" : undefined}
          onClick={() => onSelect(node)}
        >
          <span className="folder-node-name">{node.name}</span>
          <small>
            {node.directCardCount} / {node.totalCardCount}
          </small>
        </button>
      </div>
      {hasChildren && (
        <div
          id={childrenId}
          className={`folder-children ${collapsed ? "collapsed" : ""}`}
          aria-hidden={collapsed}
          inert={collapsed ? true : undefined}
        >
          <div className="folder-children-clip">
            <ul>
              {node.children.map((child) => (
                <TreeNode
                  node={child}
                  selectedId={selectedId}
                  onSelect={onSelect}
                  draggedId={draggedId}
                  dropTargetId={dropTargetId}
                  movePending={movePending}
                  canDropOn={canDropOn}
                  onDragTarget={onDragTarget}
                  onDragStart={onDragStart}
                  onDragEnd={onDragEnd}
                  onDrop={onDrop}
                  key={child.id}
                />
              ))}
            </ul>
          </div>
        </div>
      )}
    </li>
  );
}

type Action = "create" | "rename" | "move" | "delete" | null;

export function FolderPanel({
  root,
  selectedId,
  onSelect,
  onChanged,
}: {
  root: FolderNode;
  selectedId: string;
  onSelect: (node: FolderNode) => void;
  onChanged: (preferredId?: string) => Promise<void>;
}) {
  const all = useMemo(() => flattenFolders(root), [root]);
  const selected = all.find((folder) => folder.id === selectedId) ?? all[0];
  const [action, setAction] = useState<Action>(null);
  const [name, setName] = useState("");
  const [parentId, setParentId] = useState(root.id);
  const [pending, setPending] = useState(false);
  const [error, setError] = useState("");
  const [draggedId, setDraggedId] = useState<string | null>(null);
  const [dropTargetId, setDropTargetId] = useState<string | null>(null);
  const [movePending, setMovePending] = useState(false);
  const [moveError, setMoveError] = useState("");
  const nodes = useMemo(() => {
    const byId = new Map<string, FolderNode>();
    const visit = (node: FolderNode) => {
      byId.set(node.id, node);
      node.children.forEach(visit);
    };
    visit(root);
    return byId;
  }, [root]);
  const canDropOn = (targetId: string) => {
    if (!draggedId || movePending) return false;
    const source = nodes.get(draggedId);
    const target = nodes.get(targetId);
    if (!source || source.isInbox || !target) return false;
    if (source.id === target.id || source.parentId === target.id) return false;
    const containsTarget = (node: FolderNode): boolean =>
      node.children.some(
        (child) => child.id === target.id || containsTarget(child),
      );
    return !containsTarget(source);
  };
  const finishDrag = () => {
    setDraggedId(null);
    setDropTargetId(null);
  };
  const startDrag = (event: DragEvent<HTMLDivElement>, node: FolderNode) => {
    event.stopPropagation();
    if (node.isInbox || movePending) {
      event.preventDefault();
      return;
    }
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData("application/x-flashka-folder-id", node.id);
    setMoveError("");
    setDraggedId(node.id);
  };
  const moveFolder = async (sourceId: string, targetId: string) => {
    if (!canDropOn(targetId)) return;
    setMovePending(true);
    setMoveError("");
    finishDrag();
    try {
      await api.updateFolder(sourceId, { parentId: targetId });
      await onChanged(selectedId);
    } catch (e) {
      if (e instanceof RequestError && e.body?.error.message)
        setMoveError(e.body.error.message);
      else if (!(e instanceof RequestError && e.status === 401))
        setMoveError("Не удалось переместить папку. Попробуйте ещё раз.");
    } finally {
      setMovePending(false);
    }
  };
  const open = (next: Action) => {
    setAction(next);
    setError("");
    setName(next === "rename" ? selected.name : "");
    setParentId(next === "create" ? selected.id : root.id);
  };
  const submit = async (event: FormEvent) => {
    event.preventDefault();
    const fieldError =
      action === "create" || action === "rename"
        ? validateFolderName(name)
        : "";
    if (fieldError) {
      setError(fieldError);
      return;
    }
    setPending(true);
    setError("");
    try {
      if (action === "create") {
        const result = await api.createFolder({
          parentId,
          name: normalizeFolderName(name),
        });
        await onChanged(result.folder.id);
      } else if (action === "rename") {
        await api.updateFolder(selected.id, {
          name: normalizeFolderName(name),
        });
        await onChanged(selected.id);
      } else if (action === "move") {
        await api.updateFolder(selected.id, { parentId });
        await onChanged(selected.id);
      } else if (action === "delete") {
        await api.deleteFolder(selected.id);
        await onChanged(root.id);
      }
      setAction(null);
    } catch (e) {
      if (e instanceof RequestError && e.body?.error.fields?.name)
        setError(e.body.error.fields.name);
      else if (e instanceof RequestError && e.body?.error.message)
        setError(e.body.error.message);
      else if (!(e instanceof RequestError && e.status === 401))
        setError("Не удалось изменить папку. Попробуйте ещё раз.");
    } finally {
      setPending(false);
    }
  };
  return (
    <aside className="folder-panel" aria-label="Папки">
      <div className="folder-panel-title">
        <strong>Коллекция</strong>
        <span>в папке / с подпапками</span>
      </div>
      <ul className="folder-tree">
        <TreeNode
          node={root}
          selectedId={selectedId}
          onSelect={onSelect}
          draggedId={draggedId}
          dropTargetId={dropTargetId}
          movePending={movePending}
          canDropOn={canDropOn}
          onDragTarget={setDropTargetId}
          onDragStart={startDrag}
          onDragEnd={finishDrag}
          onDrop={moveFolder}
          showChildren={false}
        />
        {root.children.map((child) => (
          <TreeNode
            node={child}
            selectedId={selectedId}
            onSelect={onSelect}
            draggedId={draggedId}
            dropTargetId={dropTargetId}
            movePending={movePending}
            canDropOn={canDropOn}
            onDragTarget={setDropTargetId}
            onDragStart={startDrag}
            onDragEnd={finishDrag}
            onDrop={moveFolder}
            key={child.id}
          />
        ))}
      </ul>
      {moveError && (
        <p className="field-error folder-move-error" role="alert">
          {moveError}
        </p>
      )}
      <div className="folder-actions">
        <button className="secondary" onClick={() => open("create")}>
          <span aria-hidden="true">＋</span> Создать внутри
        </button>
        {selected.id !== root.id && (
          <>
            <button className="secondary" onClick={() => open("rename")}>
              Переименовать
            </button>
            <button className="secondary" onClick={() => open("move")}>
              Переместить
            </button>
            <button className="danger-outline" onClick={() => open("delete")}>
              Удалить
            </button>
          </>
        )}
      </div>
      {action && (
        <form className="folder-form" onSubmit={submit}>
          <strong>
            {action === "create" && "Новая папка"}
            {action === "rename" && "Новое название"}
            {action === "move" && "Новый родитель"}
            {action === "delete" && "Удалить папку?"}
          </strong>
          {(action === "create" || action === "rename") && (
            <>
              <label htmlFor="folder-name">Название</label>
              <input
                id="folder-name"
                autoFocus
                value={name}
                onChange={(e) => setName(e.target.value)}
              />
            </>
          )}
          {(action === "create" || action === "move") && (
            <>
              <label htmlFor="folder-parent">Родитель</label>
              <select
                id="folder-parent"
                value={parentId}
                onChange={(e) => setParentId(e.target.value)}
              >
                {all
                  .filter(
                    (folder) =>
                      action === "create" || folder.id !== selected.id,
                  )
                  .map((folder) => (
                    <option value={folder.id} key={folder.id}>
                      {folder.label}
                    </option>
                  ))}
              </select>
            </>
          )}
          {action === "delete" && (
            <p>Удалить можно только папку без карточек и подпапок.</p>
          )}
          {error && (
            <p className="field-error" role="alert">
              {error}
            </p>
          )}
          <div className="actions">
            <button
              className={action === "delete" ? "danger" : ""}
              disabled={pending}
            >
              {pending
                ? "Сохраняем…"
                : action === "delete"
                  ? "Удалить папку"
                  : "Сохранить"}
            </button>
            <button
              type="button"
              className="secondary"
              disabled={pending}
              onClick={() => setAction(null)}
            >
              Отмена
            </button>
          </div>
        </form>
      )}
    </aside>
  );
}

export function useFolderTree() {
  const [root, setRoot] = useState<FolderNode | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState("");
  const load = async () => {
    setLoading(true);
    setError("");
    try {
      const data = await api.folderTree();
      setRoot(data.root);
      return data.root;
    } catch (e) {
      if (!(e instanceof RequestError && e.status === 401))
        setError("Не удалось загрузить папки.");
      return null;
    } finally {
      setLoading(false);
    }
  };
  useEffect(() => {
    let active = true;
    void api
      .folderTree()
      .then((data) => {
        if (active) setRoot(data.root);
      })
      .catch((e) => {
        if (active && !(e instanceof RequestError && e.status === 401))
          setError("Не удалось загрузить папки.");
      })
      .finally(() => {
        if (active) setLoading(false);
      });
    return () => {
      active = false;
    };
  }, []);
  return { root, loading, error, load };
}
