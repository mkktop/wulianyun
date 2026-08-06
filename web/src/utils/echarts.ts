// ECharts 按需引入：仅注册项目实际使用的图表与组件，
// 将整包（~1.2MB）裁剪到 ~0.4MB。各页面统一从此入口引入 echarts。
import * as echarts from 'echarts/core'
import { LineChart, BarChart, PieChart } from 'echarts/charts'
import { GridComponent, TooltipComponent, LegendComponent } from 'echarts/components'
import { CanvasRenderer } from 'echarts/renderers'

echarts.use([
  LineChart, BarChart, PieChart,
  GridComponent, TooltipComponent, LegendComponent,
  CanvasRenderer,
])

export default echarts
export { echarts }
export type ECharts = echarts.ECharts
