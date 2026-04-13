#!/usr/bin/env node

const fs = require("fs");
const path = require("path");
const { spawnSync } = require("child_process");

function parseArgs(argv) {
  const out = {};
  for (let i = 2; i < argv.length; i += 1) {
    const key = argv[i];
    if (!key.startsWith("--")) {
      continue;
    }
    const value = argv[i + 1];
    if (value && !value.startsWith("--")) {
      out[key.slice(2)] = value;
      i += 1;
    } else {
      out[key.slice(2)] = "true";
    }
  }
  return out;
}

function loadModule(name) {
  try {
    return require(name);
  } catch (_) {
    const fallback = path.join("/root/work/node_modules", name);
    return require(fallback);
  }
}

function fileExists(p) {
  try {
    return !!p && fs.statSync(p).isFile();
  } catch (_) {
    return false;
  }
}

function selectDocxTemplate(artifactName, baseName) {
  const key = `${artifactName || ""} ${baseName || ""}`.toLowerCase();
  const templates = [
    {
      match: ["活动方案初稿", "执行方案", "plan", "draft"],
      path: "/root/工作流相关参考文件/2026小龙虾生态沙龙_执行方案.docx",
    },
    {
      match: ["详细活动流程表", "流程表", "agenda", "timeline"],
      path: "/root/工作流相关参考文件/12月份活动流程表_深圳.docx",
    },
  ];
  for (const item of templates) {
    if (item.match.some((m) => key.includes(m.toLowerCase())) && fileExists(item.path)) {
      return item.path;
    }
  }
  return "";
}

function markdownToDocxWithPandoc(inputPath, outPath, templatePath) {
  const args = [inputPath, "-f", "gfm", "-t", "docx", "-o", outPath];
  if (templatePath) {
    args.push("--reference-doc", templatePath);
  }
  const res = spawnSync("pandoc", args, { encoding: "utf8" });
  if (res.status !== 0) {
    const stderr = (res.stderr || "").trim();
    throw new Error(`pandoc failed: ${stderr || `exit ${res.status}`}`);
  }
}

function extractTitle(markdown, fallback) {
  const lines = markdown.split(/\r?\n/);
  for (const line of lines) {
    const m = line.match(/^#\s+(.+)$/);
    if (m) {
      return m[1].trim();
    }
  }
  return fallback;
}

function parseMarkdownTables(markdown) {
  const lines = markdown.split(/\r?\n/);
  const tables = [];
  let i = 0;
  while (i < lines.length - 1) {
    const header = lines[i].trim();
    const sep = lines[i + 1].trim();
    if (!header.includes("|") || !/^\|?[\s:\-|\t]+\|?$/.test(sep)) {
      i += 1;
      continue;
    }
    const rows = [];
    rows.push(lines[i]);
    i += 2;
    while (i < lines.length && lines[i].includes("|")) {
      rows.push(lines[i]);
      i += 1;
    }
    const parsed = rows
      .map((line) =>
        line
          .trim()
          .replace(/^\|/, "")
          .replace(/\|$/, "")
          .split("|")
          .map((c) => c.trim())
      )
      .filter((r) => r.length > 0 && r.some((c) => c !== ""));
    if (parsed.length > 1) {
      tables.push(parsed);
    }
  }
  return tables;
}

function markdownToDocx(markdown, inputPath, outPath, title, artifactName) {
  const templatePath = selectDocxTemplate(artifactName, path.basename(inputPath, path.extname(inputPath)));
  if (fileExists(inputPath)) {
    try {
      markdownToDocxWithPandoc(inputPath, outPath, templatePath);
      return Promise.resolve();
    } catch (_) {
      // Fallback to internal renderer when pandoc conversion fails.
    }
  }
  const docx = loadModule("docx");
  const {
    Document,
    Packer,
    Paragraph,
    HeadingLevel,
    Table,
    TableRow,
    TableCell,
    WidthType,
    BorderStyle,
    TextRun,
  } = docx;

  const border = { style: BorderStyle.SINGLE, size: 1, color: "C8C8C8" };
  const lines = markdown.split(/\r?\n/);
  const blocks = [];
  const tables = parseMarkdownTables(markdown);

  let tableIndex = 0;
  for (let i = 0; i < lines.length; i += 1) {
    const line = lines[i];
    const trimmed = line.trim();
    if (trimmed === "") {
      blocks.push(new Paragraph({ text: "" }));
      continue;
    }
    if (/^#{1,3}\s+/.test(trimmed)) {
      const level = trimmed.match(/^#+/)[0].length;
      const text = trimmed.replace(/^#{1,3}\s+/, "").trim();
      const heading =
        level === 1
          ? HeadingLevel.HEADING_1
          : level === 2
            ? HeadingLevel.HEADING_2
            : HeadingLevel.HEADING_3;
      blocks.push(new Paragraph({ text, heading }));
      continue;
    }
    if (line.includes("|") && i + 1 < lines.length && /^\|?[\s:\-|\t]+\|?$/.test(lines[i + 1].trim())) {
      const table = tables[tableIndex];
      tableIndex += 1;
      if (!table) {
        continue;
      }
      const colCount = table[0].length;
      const width = Math.floor(9360 / Math.max(1, colCount));
      blocks.push(
        new Table({
          width: { size: 9360, type: WidthType.DXA },
          columnWidths: Array(colCount).fill(width),
          rows: table.map((r, ridx) =>
            new TableRow({
              children: r.map((cell) =>
                new TableCell({
                  width: { size: width, type: WidthType.DXA },
                  borders: { top: border, bottom: border, left: border, right: border },
                  children: [
                    new Paragraph({
                      children: [new TextRun({ text: cell || " ", bold: ridx === 0 })],
                    }),
                  ],
                })
              ),
            })
          ),
        })
      );
      i += 1;
      while (i + 1 < lines.length && lines[i + 1].includes("|")) {
        i += 1;
      }
      continue;
    }
    blocks.push(new Paragraph({ text: trimmed }));
  }

  const doc = new Document({
    sections: [
      {
        children: [
          new Paragraph({ text: title, heading: HeadingLevel.TITLE }),
          ...blocks,
        ],
      },
    ],
  });

  return Packer.toBuffer(doc).then((buf) => fs.writeFileSync(outPath, buf));
}

function markdownTableToXlsx(markdown, outPath) {
  const xlsx = loadModule("xlsx");
  const tables = parseMarkdownTables(markdown);
  const wb = xlsx.utils.book_new();
  if (tables.length === 0) {
    const rows = markdown
      .split(/\r?\n/)
      .map((line) => [line])
      .filter((r) => r[0].trim() !== "");
    const ws = xlsx.utils.aoa_to_sheet(rows.length ? rows : [[""]]);
    xlsx.utils.book_append_sheet(wb, ws, "内容");
  } else {
    tables.forEach((table, idx) => {
      const ws = xlsx.utils.aoa_to_sheet(table);
      xlsx.utils.book_append_sheet(wb, ws, `表${idx + 1}`);
    });
  }
  xlsx.writeFile(wb, outPath);
}

async function main() {
  const args = parseArgs(process.argv);
  const input = path.resolve(args.input || "");
  if (!input || !fs.existsSync(input)) {
    throw new Error(`input not found: ${input}`);
  }
  const outDir = path.resolve(args.outDir || path.dirname(input));
  const baseName = path.basename(input, path.extname(input));
  const artifactName = (args.artifactName || "").trim();
  const markdown = fs.readFileSync(input, "utf8");
  const title = extractTitle(markdown, baseName);
  const docxPath = path.join(outDir, `${baseName}.docx`);
  const xlsxPath = path.join(outDir, `${baseName}.xlsx`);

  await markdownToDocx(markdown, input, docxPath, title, artifactName);
  const tables = parseMarkdownTables(markdown);
  if (tables.length > 0) {
    markdownTableToXlsx(markdown, xlsxPath);
  }

  const files = [{ type: "docx", path: docxPath }];
  if (tables.length > 0) {
    files.push({ type: "xlsx", path: xlsxPath });
  }
  process.stdout.write(JSON.stringify({ ok: true, files }));
}

main().catch((err) => {
  process.stderr.write(String(err && err.stack ? err.stack : err));
  process.exit(1);
});
