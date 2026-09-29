// 引擎产出的结果。桌面和 TUI 读的是同一份。没有坐标，没有写好的解说，界面自己排、自己说。

const S = {
  web: "src/web/orderRoute.ts#orderRoute#postOrder#2",
  cron: "src/jobs/Reconcile.ts#Reconcile#run#0",
  create: "src/order/OrderService.ts#OrderService#create#1",
  reserve: "src/stock/Inventory.ts#Inventory#reserve#1",
  lock: "src/stock/StockRepo.ts#StockRepo#lock#1",
  charge: "src/pay/Payment.ts#Payment#charge#1",
  db: "ext:pg",
  pay: "ext:stripe"
};

const result = {
  repo: "/Users/fitz/work/shop",
  sessions: [
    {
      agent: "Claude",
      sid: "c3e1a0b2-5f4d-4e8a-9b61-0d2c7f4a1e93",
      title: "下单改走库存预占",
      baseline: "8c1a40e",
      bound: "start",
      source: "~/.claude/projects/-Users-fitz-work-shop/c3e1a0b2-5f4d-4e8a-9b61-0d2c7f4a1e93.jsonl",
      turns: [
        {
          id: "t2",
          at: "14:02",
          prompt: "下单别直接扣款，先去库存预占，锁库存要落库。",
          reply: "下单改成先预占库存，新增 StockRepo.lock 落库。支付改到发货时再扣。",
          edits: [
            { tool: "Edit", path: "src/order/OrderService.ts", ok: true },
            { tool: "Edit", path: "src/stock/Inventory.ts", ok: false, reason: "工具报错：old_string 不唯一，这笔没生效，跳过" },
            { tool: "Edit", path: "src/stock/Inventory.ts", ok: true },
            { tool: "Edit", path: "src/stock/StockRepo.ts", ok: true }
          ],
          stars: [
            { id: S.web, label: "页面下单", kind: "entry" },
            { id: S.cron, label: "每晚对账", kind: "entry" },
            {
              id: S.create, label: "OrderService.create", kind: "fn", mark: "edit",
              body: { added: ["await this.inventory.reserve(order);"], removed: ["await this.payment.charge(order);"] }
            },
            {
              id: S.reserve, label: "Inventory.reserve", kind: "fn", mark: "edit",
              body: { added: ["await this.stockRepo.lock(sku);"], removed: [] }
            },
            {
              id: S.lock, label: "StockRepo.lock", kind: "fn", mark: "add",
              body: { added: ["async lock(sku: string) {", "  return this.db.save({ sku, at: Date.now() });", "}"], removed: [] }
            },
            { id: S.charge, label: "Payment.charge", kind: "fn" },
            { id: S.db, label: "数据库", kind: "leaf" },
            { id: S.pay, label: "支付渠道", kind: "leaf" }
          ],
          calls: [
            { key: `${S.web}>${S.create}#0`, from: S.web, to: S.create, change: "none", text: "orderService.create(order)" },
            { key: `${S.create}>${S.reserve}#0`, from: S.create, to: S.reserve, change: "add", after: "this.inventory.reserve(order)" },
            { key: `${S.create}>${S.charge}#0`, from: S.create, to: S.charge, change: "del", before: "this.payment.charge(order)" },
            { key: `${S.reserve}>${S.lock}#0`, from: S.reserve, to: S.lock, change: "add", after: "this.stockRepo.lock(sku)" },
            { key: `${S.lock}>${S.db}#0`, from: S.lock, to: S.db, change: "add", after: "this.db.save({ sku, at: Date.now() })" },
            { key: `${S.cron}>${S.reserve}#0`, from: S.cron, to: S.reserve, change: "none", uncertain: true, text: "this.inventory.reserve(sku)" },
            { key: `${S.charge}>${S.pay}#0`, from: S.charge, to: S.pay, change: "none", text: "stripe.charges.create(req)" }
          ],
          chains: [
            { stars: [S.web, S.create, S.reserve, S.lock, S.db] },
            { stars: [S.cron, S.reserve, S.lock, S.db] },
            { stars: [S.web, S.create, S.charge, S.pay], dashed: true }
          ],
          omitted: 0,
          errors: []
        },
        {
          id: "t1",
          at: "13:40",
          prompt: "下单前校验价格，小于 0 就拒绝。",
          reply: "在 OrderService.create 开头加了价格校验，小于 0 抛 PriceError。",
          edits: [
            { tool: "Edit", path: "src/order/OrderService.ts", ok: true }
          ],
          stars: [
            { id: S.web, label: "页面下单", kind: "entry" },
            {
              id: S.create, label: "OrderService.create", kind: "fn", mark: "edit",
              body: { added: ["if (order.price < 0) throw new PriceError(order);"], removed: [] }
            },
            { id: S.charge, label: "Payment.charge", kind: "fn" },
            { id: S.pay, label: "支付渠道", kind: "leaf" }
          ],
          calls: [
            { key: `${S.web}>${S.create}#0`, from: S.web, to: S.create, change: "none", text: "orderService.create(order)" },
            { key: `${S.create}>${S.charge}#0`, from: S.create, to: S.charge, change: "none", text: "this.payment.charge(order)" },
            { key: `${S.charge}>${S.pay}#0`, from: S.charge, to: S.pay, change: "none", text: "stripe.charges.create(req)" }
          ],
          chains: [
            { stars: [S.web, S.create, S.charge, S.pay] }
          ],
          omitted: 0,
          errors: []
        }
      ]
    },
    {
      agent: "Codex",
      sid: "01a0d64a-9b2a-79b2-ba3b-07fb4441cafa",
      title: "对账加重试",
      baseline: "8c1a40e",
      bound: "late",
      source: "~/.codex/sessions/2026/09/25/rollout-2026-09-25T13-50-03-01a0d64a-9b2a-79b2-ba3b-07fb4441cafa.jsonl",
      turns: [
        {
          id: "t1",
          at: "13:55",
          prompt: "对账时预占失败就重试三次。",
          reply: "对账里预占失败会重试，最多三次。",
          edits: [
            { tool: "apply_patch", path: "src/jobs/Reconcile.ts", ok: true },
            { tool: "apply_patch", path: "src/stock/Inventory.ts", ok: true }
          ],
          stars: [
            {
              id: S.cron, label: "每晚对账", kind: "entry", mark: "edit", unknownCalls: true,
              body: {
                added: ["for (let i = 0; i < 3; i += 1) {", "  if (await this.inventory.reserve(sku, { retry: i })) break;", "}"],
                removed: ["await this.inventory.reserve(sku);"]
              }
            },
            { id: S.reserve, label: "Inventory.reserve", kind: "fn" }
          ],
          calls: [
            {
              key: `${S.cron}>${S.reserve}#0`, from: S.cron, to: S.reserve, change: "edit", uncertain: true,
              before: "this.inventory.reserve(sku)", after: "this.inventory.reserve(sku, { retry: i })"
            }
          ],
          chains: [
            { stars: [S.cron, S.reserve], broken: "src/stock/Inventory.ts" }
          ],
          omitted: 0,
          errors: [
            {
              path: "src/stock/Inventory.ts",
              reason: "从记录末尾倒放第 2 笔 apply_patch 时，补丁里加上的行在当前正文里找不到。这个文件之后被别的会话改过，停在这里，不往下编链。"
            }
          ]
        }
      ]
    }
  ]
};
