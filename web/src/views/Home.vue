<template>
  <div class="home">
    <!-- ===== 顶栏 ===== -->
    <header class="topbar">
      <div class="topbar-inner">
        <a class="brand" href="/" title="KK物联云">
          <svg viewBox="0 0 24 24" class="brand-logo" aria-hidden="true">
            <rect x="1" y="1" width="22" height="22" rx="6" fill="#1677ff" />
            <path d="M7 11.5a7 7 0 0 1 10 0M9.2 14a4.2 4.2 0 0 1 5.6 0" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round" />
            <circle cx="12" cy="17" r="1.6" fill="#fff" />
          </svg>
          <span class="brand-name">KK物联云</span>
        </a>
        <nav class="nav">
          <a href="#features">核心能力</a>
          <a href="#workflow">接入流程</a>
          <a href="#scenarios">应用场景</a>
          <a href="#dev">开发者资源</a>
        </nav>
        <div class="topbar-actions">
          <a class="btn btn-ghost btn-sm" href="/developer/" target="_blank" rel="noopener">开发文档</a>
          <a class="btn btn-primary btn-sm" href="/console/overview">进入控制台</a>
        </div>
      </div>
    </header>

    <!-- ===== Hero ===== -->
    <section class="hero">
      <div class="hero-inner">
        <div class="hero-text">
          <div class="hero-badge"><span class="dot"></span>一站式物联网云平台</div>
          <h1>设备接入 · 数据管理<br />实时监控，<span class="grad">一云搞定</span></h1>
          <p class="hero-sub">
            面向设备厂商与集成商的物联网云平台，支持 MQTT / TCP / HTTP / Modbus 多协议接入，
            物模型、规则引擎、告警中心、OTA 升级全链路管理，助力设备快速上云。
          </p>
          <div class="hero-actions">
            <a class="btn btn-primary" href="/console/overview">进入控制台</a>
            <a class="btn btn-ghost" href="/developer/" target="_blank" rel="noopener">查看开发文档</a>
          </div>
        </div>

        <!-- 设备拓扑示意（纯 SVG 绘制） -->
        <div class="hero-visual">
          <svg viewBox="0 0 560 420" class="topology" role="img" aria-label="设备接入拓扑示意">
            <!-- 连线（数据上行 / 指令下行） -->
            <g class="flow up">
              <path d="M106 116 Q 180 140 245 172" />
              <path d="M106 304 Q 180 280 245 228" />
              <path d="M454 116 Q 380 140 315 172" />
              <path d="M454 304 Q 380 280 315 228" />
            </g>

            <!-- 中心云节点 -->
            <g class="cloud-node">
              <circle cx="245" cy="185" r="30" />
              <circle cx="280" cy="170" r="36" />
              <circle cx="318" cy="185" r="30" />
              <rect x="238" y="180" width="84" height="52" rx="24" />
              <g class="cloud-icon">
                <path d="M272 201 a11 11 0 0 1 16 0" />
                <path d="M275.5 205 a6 6 0 0 1 9 0" />
                <circle cx="280" cy="211" r="2" />
              </g>
            </g>

            <!-- 设备节点 -->
            <g class="node">
              <circle cx="80" cy="90" r="26" />
              <g class="node-icon" transform="translate(80 90)">
                <rect x="-7" y="-13" width="14" height="26" rx="3" />
                <line x1="-3" y1="9" x2="3" y2="9" />
              </g>
              <text x="80" y="138">手机 / App</text>
            </g>
            <g class="node">
              <circle cx="480" cy="90" r="26" />
              <g class="node-icon" transform="translate(480 90)">
                <rect x="-9" y="-7" width="18" height="13" rx="2" />
                <line x1="-4" y1="9" x2="4" y2="9" />
                <circle cx="0" cy="-13" r="3" />
              </g>
              <text x="480" y="138">边缘网关</text>
            </g>
            <g class="node">
              <circle cx="80" cy="330" r="26" />
              <g class="node-icon" transform="translate(80 330)">
                <rect x="-9" y="-8" width="18" height="16" rx="2" />
                <circle cx="-4" cy="0" r="1.5" />
                <circle cx="0" cy="0" r="1.5" />
                <circle cx="4" cy="0" r="1.5" />
              </g>
              <text x="80" y="378">PLC / 仪表</text>
            </g>
            <g class="node">
              <circle cx="480" cy="330" r="26" />
              <g class="node-icon" transform="translate(480 330)">
                <path d="M0 -13 C 5 -5 8 0 8 5 a8 8 0 1 1 -16 0 C -8 0 -5 -5 0 -13z" />
              </g>
              <text x="480" y="378">环境传感器</text>
            </g>
          </svg>
        </div>
      </div>

      <!-- 能力数据条 -->
      <div class="stats">
        <div class="stat" v-for="s in stats" :key="s.value">
          <div class="stat-value">{{ s.value }}</div>
          <div class="stat-label">{{ s.label }}</div>
        </div>
      </div>
    </section>

    <!-- ===== 核心能力 ===== -->
    <section id="features" class="section">
      <div class="section-head">
        <span class="tag">核心能力</span>
        <h2>开箱即用，覆盖设备全生命周期</h2>
        <p>从设备接入、数据采集到应用集成，一站式管理</p>
      </div>
      <div class="card-grid">
        <div class="feature-card" v-for="f in features" :key="f.title">
          <div class="feature-icon" :style="{ background: f.bg, color: f.color }">
            <el-icon :size="26"><component :is="f.icon" /></el-icon>
          </div>
          <h3>{{ f.title }}</h3>
          <p>{{ f.desc }}</p>
        </div>
      </div>
    </section>

    <!-- ===== 接入流程 ===== -->
    <section id="workflow" class="section alt">
      <div class="section-head">
        <span class="tag">接入流程</span>
        <h2>四步完成设备上云</h2>
        <p>无需了解底层协议细节，控制台引导式完成接入</p>
      </div>
      <div class="steps">
        <div class="step" v-for="(s, i) in steps" :key="s.title">
          <div class="step-num">{{ i + 1 }}</div>
          <h3>{{ s.title }}</h3>
          <p>{{ s.desc }}</p>
        </div>
      </div>
    </section>

    <!-- ===== 应用场景 ===== -->
    <section id="scenarios" class="section">
      <div class="section-head">
        <span class="tag">应用场景</span>
        <h2>多行业实践，随需而变</h2>
        <p>灵活的平台能力，适配各类物联网业务场景</p>
      </div>
      <div class="card-grid">
        <div class="scenario-card" v-for="s in scenarios" :key="s.title">
          <div class="scenario-icon">
            <el-icon :size="30"><component :is="s.icon" /></el-icon>
          </div>
          <h3>{{ s.title }}</h3>
          <p>{{ s.desc }}</p>
        </div>
      </div>
    </section>

    <!-- ===== 开发者资源 ===== -->
    <section id="dev" class="section alt">
      <div class="section-head">
        <span class="tag">开发者资源</span>
        <h2>文档、接口与调试工具一应俱全</h2>
        <p>完善的开发者生态，让接入与调试更高效</p>
      </div>
      <div class="card-grid four">
        <a class="dev-card" v-for="d in devTools" :key="d.title" :href="d.href" :target="d.external ? '_blank' : undefined" :rel="d.external ? 'noopener' : undefined">
          <div class="dev-icon">
            <el-icon :size="24"><component :is="d.icon" /></el-icon>
          </div>
          <h3>{{ d.title }}</h3>
          <p>{{ d.desc }}</p>
          <span class="dev-more">了解更多 →</span>
        </a>
      </div>
      <div class="cta">
        <h3>准备好开始了吗？</h3>
        <p>免费接入你的第一台设备，开启万物互联之旅</p>
        <a class="btn btn-primary" href="/console/overview">立即进入控制台</a>
      </div>
    </section>

    <!-- ===== 页脚 ===== -->
    <footer class="footer">
      <div class="footer-inner">
        <div class="footer-brand">
          <div class="brand">
            <svg viewBox="0 0 24 24" class="brand-logo" aria-hidden="true">
              <rect x="1" y="1" width="22" height="22" rx="6" fill="#1677ff" />
              <path d="M7 11.5a7 7 0 0 1 10 0M9.2 14a4.2 4.2 0 0 1 5.6 0" fill="none" stroke="#fff" stroke-width="1.8" stroke-linecap="round" />
              <circle cx="12" cy="17" r="1.6" fill="#fff" />
            </svg>
            <span>KK物联云</span>
          </div>
          <p>一站式物联网云平台，多协议设备接入、数据管理与应用集成。</p>
        </div>
        <div class="footer-cols">
          <div class="footer-col">
            <h4>平台</h4>
            <a href="/console/overview">进入控制台</a>
            <a href="/console/products">产品管理</a>
            <a href="/console/devices">设备管理</a>
            <a href="/console/rules">规则引擎</a>
          </div>
          <div class="footer-col">
            <h4>开发者</h4>
            <a href="/developer/" target="_blank" rel="noopener">开发文档</a>
            <a href="/console/apps">开放 API</a>
            <a href="/console/tools/simulator">设备模拟器</a>
            <a href="/console/tools/mqtt-debug">MQTT 调试台</a>
          </div>
          <div class="footer-col">
            <h4>更多</h4>
            <a href="/screen">可视化大屏</a>
            <a href="/console/alarms">告警中心</a>
            <a href="/console/ota">OTA 升级</a>
          </div>
        </div>
      </div>
      <div class="footer-bottom">
        <span>© 2026 KK物联云 · 保留所有权利</span>
      </div>
    </footer>
  </div>
</template>

<script setup lang="ts">
const stats = [
  { value: '4', label: '大接入协议' },
  { value: '3', label: '种数据模式' },
  { value: '99.9%', label: '服务可用性' },
  { value: '7×24', label: '稳定运行' }
]

const features = [
  {
    icon: 'Connection', color: '#1677ff', bg: '#e8f1ff',
    title: '多协议设备接入',
    desc: 'MQTT / TCP 透传 / HTTP 直传 / Modbus 云端轮询四种接入方式，一机一密、一型一密双密钥模式，动态 Token 认证更安全。'
  },
  {
    icon: 'Grid', color: '#00a870', bg: '#e6f7ef',
    title: '物模型管理',
    desc: '标准物模型、透传脚本解析、Modbus 点位表三种数据模式，属性、事件、服务统一建模，数据格式标准化。'
  },
  {
    icon: 'SetUp', color: '#e6a23c', bg: '#fdf3e2',
    title: '规则引擎',
    desc: '可视化规则编排，数据过滤清洗、多设备联动、消息转发，规则实时生效，秒级响应业务变化。'
  },
  {
    icon: 'BellFilled', color: '#f56c6c', bg: '#fdeeee',
    title: '告警中心',
    desc: '实时告警监测与推送，分级告警处理，消息轨迹全程可追溯，异常第一时间触达。'
  },
  {
    icon: 'UploadFilled', color: '#722ed1', bg: '#f4edfd',
    title: 'OTA 升级',
    desc: '固件版本管理与批量升级任务，升级进度实时跟踪，设备能力持续进化。'
  },
  {
    icon: 'Link', color: '#0ea5a5', bg: '#e3f7f7',
    title: '开放 API',
    desc: 'OpenAPI 应用接入，设备数据读写、指令下发能力开放，与业务系统无缝集成。'
  }
]

const steps = [
  { title: '创建产品', desc: '选择接入协议与数据模式，一型一密支持免预注册动态接入' },
  { title: '定义物模型', desc: '配置属性、事件、服务，或编写透传解析脚本完成数据建模' },
  { title: '接入设备', desc: '通过 MQTT / TCP / HTTP 接入设备，动态 Token 换取连接凭证' },
  { title: '数据应用', desc: '规则引擎、告警、OTA、开放 API 全链路打通，数据驱动业务' }
]

const scenarios = [
  {
    icon: 'House',
    title: '智能家居',
    desc: '全屋设备统一接入管理，场景联动、远程控制，构建智慧家庭生态。'
  },
  {
    icon: 'Cpu',
    title: '工业设备监控',
    desc: 'PLC / 仪表数据云端采集，Modbus 点位表免开发，设备状态实时监控与远程运维。'
  },
  {
    icon: 'Sunny',
    title: '智慧农业',
    desc: '环境传感器数据采集与阈值告警，远程灌溉控制，助力精细化种植。'
  }
]

const devTools = [
  {
    icon: 'Document', href: '/developer/', external: true,
    title: '开发文档',
    desc: '接入协议、物模型、OpenAPI 完整参考，快速上手接入。'
  },
  {
    icon: 'Key', href: '/console/apps',
    title: '开放 API',
    desc: '创建应用获取密钥，通过开放接口读写设备数据。'
  },
  {
    icon: 'Monitor', href: '/console/tools/simulator',
    title: '设备模拟器',
    desc: '无需硬件，在线模拟设备上报数据、接收指令。'
  },
  {
    icon: 'ChatDotSquare', href: '/console/tools/mqtt-debug',
    title: 'MQTT 调试台',
    desc: '在线调试设备上下行消息，排查接入问题。'
  }
]
</script>

<style scoped>
:global(html) { scroll-behavior: smooth; }

.home {
  min-height: 100%;
  background: #fff;
  color: #22304a;
  font-size: 15px;
  line-height: 1.7;
}

/* ---------- 通用 ---------- */
.btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  padding: 11px 26px;
  border-radius: 8px;
  font-size: 15px;
  font-weight: 500;
  text-decoration: none;
  transition: all .2s;
  cursor: pointer;
}
.btn-sm { padding: 8px 18px; font-size: 14px; }
.btn-primary {
  background: linear-gradient(135deg, #1677ff, #409eff);
  color: #fff;
  box-shadow: 0 6px 16px rgba(22, 119, 255, .28);
}
.btn-primary:hover { transform: translateY(-1px); box-shadow: 0 8px 20px rgba(22, 119, 255, .38); }
.btn-ghost {
  background: #fff;
  color: #1677ff;
  border: 1px solid #c9dcff;
}
.btn-ghost:hover { border-color: #1677ff; background: #f0f6ff; }

.section { padding: 84px 24px 96px; }
.section.alt { background: #f6f9ff; }
.section-head { max-width: 760px; margin: 0 auto 52px; text-align: center; }
.section-head .tag {
  display: inline-block;
  padding: 4px 14px;
  border-radius: 20px;
  background: #e8f1ff;
  color: #1677ff;
  font-size: 13px;
  font-weight: 500;
  letter-spacing: 2px;
}
.section-head h2 { margin: 14px 0 8px; font-size: 30px; color: #12234a; }
.section-head p { margin: 0; color: #71809c; }
#features, #workflow, #scenarios, #dev { scroll-margin-top: 64px; }

.card-grid {
  max-width: 1160px;
  margin: 0 auto;
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 22px;
}
.card-grid.four { grid-template-columns: repeat(4, 1fr); }

/* ---------- 顶栏 ---------- */
.topbar {
  position: sticky;
  top: 0;
  z-index: 100;
  background: rgba(255, 255, 255, .86);
  backdrop-filter: blur(10px);
  border-bottom: 1px solid #e8f0ff;
}
.topbar-inner {
  max-width: 1160px;
  margin: 0 auto;
  height: 64px;
  padding: 0 24px;
  display: flex;
  align-items: center;
  gap: 40px;
}
.brand { display: flex; align-items: center; gap: 10px; text-decoration: none; }
.brand-logo { width: 28px; height: 28px; flex-shrink: 0; }
.brand-name { font-size: 19px; font-weight: 700; color: #12234a; }
.nav { display: flex; gap: 30px; margin-left: 8px; }
.nav a { color: #44506b; text-decoration: none; font-size: 15px; transition: color .2s; }
.nav a:hover { color: #1677ff; }
.topbar-actions { margin-left: auto; display: flex; gap: 12px; }

/* ---------- Hero ---------- */
.hero {
  position: relative;
  overflow: hidden;
  background: linear-gradient(180deg, #e8f2ff 0%, #f7fbff 100%);
}
.hero::before {
  content: '';
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(22, 119, 255, .05) 1px, transparent 1px),
    linear-gradient(90deg, rgba(22, 119, 255, .05) 1px, transparent 1px);
  background-size: 44px 44px;
  mask-image: radial-gradient(ellipse at 30% 20%, black 10%, transparent 70%);
  pointer-events: none;
}
.hero::after {
  content: '';
  position: absolute;
  top: -160px;
  right: -120px;
  width: 520px;
  height: 520px;
  border-radius: 50%;
  background: radial-gradient(circle, rgba(22, 119, 255, .12), transparent 65%);
  pointer-events: none;
}
.hero-inner {
  position: relative;
  max-width: 1160px;
  margin: 0 auto;
  padding: 76px 24px 60px;
  display: grid;
  grid-template-columns: 1.05fr .95fr;
  gap: 40px;
  align-items: center;
}
.hero-badge {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  padding: 6px 16px;
  border-radius: 20px;
  background: rgba(22, 119, 255, .08);
  color: #1677ff;
  font-size: 14px;
  font-weight: 500;
}
.hero-badge .dot {
  width: 8px; height: 8px; border-radius: 50%;
  background: #1677ff;
  box-shadow: 0 0 0 4px rgba(22, 119, 255, .15);
}
.hero-text h1 {
  margin: 22px 0 18px;
  font-size: 42px;
  line-height: 1.35;
  letter-spacing: 1px;
  color: #0d1f42;
}
.hero-text h1 .grad {
  background: linear-gradient(90deg, #1677ff, #00b8d9);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.hero-sub { margin: 0 0 34px; color: #5a6a8a; font-size: 16px; }
.hero-actions { display: flex; gap: 14px; }

/* 拓扑图 */
.hero-visual { display: flex; justify-content: center; }
.topology { width: 100%; max-width: 560px; height: auto; }
.topology .flow path {
  fill: none;
  stroke: #7aa7ff;
  stroke-width: 2;
  stroke-dasharray: 6 8;
  animation: flow 1.4s linear infinite;
}
.topology .flow.up path { animation-direction: reverse; }
@keyframes flow { to { stroke-dashoffset: -28; } }
.topology .cloud-node circle,
.topology .cloud-node rect {
  fill: #fff;
  stroke: #1677ff;
  stroke-width: 1.6;
}
.topology .cloud-node .cloud-icon path {
  fill: none;
  stroke: #1677ff;
  stroke-width: 2;
  stroke-linecap: round;
}
.topology .cloud-node .cloud-icon circle { fill: #1677ff; }
.topology .node circle {
  fill: #fff;
  stroke: #cfe0ff;
  stroke-width: 1.5;
}
.topology .node text {
  fill: #5a6a8a;
  font-size: 12.5px;
  text-anchor: middle;
}
.topology .node-icon path,
.topology .node-icon rect,
.topology .node-icon line {
  fill: none;
  stroke: #1677ff;
  stroke-width: 1.8;
  stroke-linecap: round;
  stroke-linejoin: round;
}
.topology .node-icon circle { fill: #1677ff; }

/* 数据条 */
.stats {
  position: relative;
  max-width: 1160px;
  margin: 0 auto;
  padding: 0 24px 56px;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 16px;
}
.stat {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  gap: 4px;
  padding: 22px 10px;
  border-radius: 12px;
  background: #fff;
  border: 1px solid #e4eeff;
  box-shadow: 0 8px 24px rgba(22, 119, 255, .08);
}
.stat-value {
  font-size: 32px;
  font-weight: 700;
  background: linear-gradient(90deg, #1677ff, #00b8d9);
  -webkit-background-clip: text;
  background-clip: text;
  color: transparent;
}
.stat-label { color: #71809c; font-size: 14px; }

/* ---------- 核心能力 ---------- */
.feature-card {
  padding: 30px 28px;
  border-radius: 14px;
  background: #fff;
  border: 1px solid #e8f0ff;
  transition: all .25s;
}
.feature-card:hover { transform: translateY(-4px); box-shadow: 0 12px 30px rgba(22, 119, 255, .12); border-color: #cfe0ff; }
.feature-icon {
  width: 52px; height: 52px;
  border-radius: 12px;
  display: flex; align-items: center; justify-content: center;
  margin-bottom: 18px;
}
.feature-card h3 { margin: 0 0 10px; font-size: 18px; color: #12234a; }
.feature-card p { margin: 0; color: #71809c; font-size: 14px; }

/* ---------- 接入流程 ---------- */
.steps {
  max-width: 1160px;
  margin: 0 auto;
  display: grid;
  grid-template-columns: repeat(4, 1fr);
  gap: 22px;
  counter-reset: step;
}
.step {
  position: relative;
  padding: 30px 26px;
  border-radius: 14px;
  background: #fff;
  border: 1px solid #e8f0ff;
  text-align: center;
}
.step-num {
  width: 44px; height: 44px;
  margin: 0 auto 16px;
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #1677ff, #409eff);
  color: #fff;
  font-size: 18px;
  font-weight: 700;
  box-shadow: 0 6px 14px rgba(22, 119, 255, .3);
}
.step::after {
  content: '';
  position: absolute;
  top: 51px;
  right: -22px;
  width: 22px;
  border-top: 2px dashed #bcd4ff;
}
.step:last-child::after { display: none; }
.step h3 { margin: 0 0 8px; font-size: 17px; color: #12234a; }
.step p { margin: 0; color: #71809c; font-size: 13.5px; }

/* ---------- 应用场景 ---------- */
.scenario-card {
  padding: 34px 30px;
  border-radius: 14px;
  background: linear-gradient(180deg, #f4f8ff, #fff);
  border: 1px solid #e8f0ff;
  text-align: center;
  transition: all .25s;
}
.scenario-card:hover { transform: translateY(-4px); box-shadow: 0 12px 30px rgba(22, 119, 255, .12); }
.scenario-icon {
  width: 64px; height: 64px;
  margin: 0 auto 18px;
  border-radius: 50%;
  display: flex; align-items: center; justify-content: center;
  background: linear-gradient(135deg, #e8f1ff, #d9ecff);
  color: #1677ff;
}
.scenario-card h3 { margin: 0 0 10px; font-size: 19px; color: #12234a; }
.scenario-card p { margin: 0; color: #71809c; font-size: 14px; }

/* ---------- 开发者资源 ---------- */
.dev-card {
  padding: 28px 24px;
  border-radius: 14px;
  background: #fff;
  border: 1px solid #e8f0ff;
  text-decoration: none;
  display: block;
  transition: all .25s;
}
.dev-card:hover { transform: translateY(-4px); box-shadow: 0 12px 30px rgba(22, 119, 255, .12); border-color: #cfe0ff; }
.dev-icon {
  width: 48px; height: 48px;
  border-radius: 10px;
  display: flex; align-items: center; justify-content: center;
  background: #e8f1ff;
  color: #1677ff;
  margin-bottom: 16px;
}
.dev-card h3 { margin: 0 0 8px; font-size: 17px; color: #12234a; }
.dev-card p { margin: 0 0 14px; color: #71809c; font-size: 13.5px; }
.dev-more { color: #1677ff; font-size: 14px; font-weight: 500; }

/* CTA */
.cta {
  max-width: 1160px;
  margin: 64px auto 0;
  padding: 48px 40px;
  border-radius: 16px;
  text-align: center;
  background: linear-gradient(135deg, #0d2b6b 0%, #1466c8 55%, #0a9bd1 100%);
  color: #fff;
  box-shadow: 0 16px 40px rgba(13, 43, 107, .3);
}
.cta h3 { margin: 0 0 8px; font-size: 26px; }
.cta p { margin: 0 0 24px; color: rgba(255, 255, 255, .8); }
.cta .btn-primary {
  background: #fff;
  color: #0d2b6b;
  box-shadow: 0 6px 16px rgba(0, 0, 0, .18);
}
.cta .btn-primary:hover { box-shadow: 0 8px 22px rgba(0, 0, 0, .26); }

/* ---------- 页脚 ---------- */
.footer { background: #0d1f42; color: #8fa3c8; }
.footer-inner {
  max-width: 1160px;
  margin: 0 auto;
  padding: 56px 24px 40px;
  display: grid;
  grid-template-columns: 1.2fr 2fr;
  gap: 48px;
}
.footer-brand .brand { margin-bottom: 14px; }
.footer-brand .brand-name { color: #fff; }
.footer-brand p { margin: 0; font-size: 14px; line-height: 1.9; }
.footer-cols { display: grid; grid-template-columns: repeat(3, 1fr); gap: 24px; }
.footer-col h4 { margin: 0 0 14px; color: #fff; font-size: 15px; }
.footer-col a {
  display: block;
  margin-bottom: 10px;
  color: #8fa3c8;
  text-decoration: none;
  font-size: 14px;
  transition: color .2s;
}
.footer-col a:hover { color: #5b9dff; }
.footer-bottom {
  border-top: 1px solid rgba(255, 255, 255, .08);
  padding: 18px 24px;
  text-align: center;
  font-size: 13px;
  color: #63789f;
}

/* ---------- 响应式 ---------- */
@media (max-width: 1080px) {
  .card-grid { grid-template-columns: repeat(2, 1fr); }
  .card-grid.four { grid-template-columns: repeat(2, 1fr); }
  .steps { grid-template-columns: repeat(2, 1fr); }
  .step::after { display: none; }
}
@media (max-width: 900px) {
  .hero-inner { grid-template-columns: 1fr; padding-top: 56px; text-align: center; }
  .hero-actions { justify-content: center; }
  .hero-text h1 { font-size: 34px; }
  .nav { display: none; }
  .stats { grid-template-columns: repeat(2, 1fr); }
  .footer-inner { grid-template-columns: 1fr; }
}
@media (max-width: 640px) {
  .section { padding: 60px 16px 72px; }
  .card-grid, .card-grid.four { grid-template-columns: 1fr; }
  .steps { grid-template-columns: 1fr; }
  .topbar-actions .btn-ghost { display: none; }
  .cta { padding: 36px 20px; }
}
@media (prefers-reduced-motion: reduce) {
  :global(html) { scroll-behavior: auto; }
  .topology .flow path { animation: none; }
}
</style>
