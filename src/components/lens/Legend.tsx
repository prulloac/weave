const TYPE_COLORS: Record<string, string> = {
	Concept: '#3b82f6',
	Table: '#10b981',
	Seed: '#f59e0b',
	Lens: '#8b5cf6',
}

const EDGE_STYLES = [
	{ label: 'Link', dashed: false },
	{ label: 'Related', dashed: true },
]

interface LegendProps {
	types: string[]
}

export default function Legend({ types }: LegendProps) {
	return (
		<div data-testid="legend">
			<h3>Legend</h3>
			<div data-testid="legend-types">
				<h4>Node Types</h4>
				<ul>
					{types.map((type) => (
						<li key={type} data-testid="legend-entry">
							<span
								style={{
									display: 'inline-block',
									width: '12px',
									height: '12px',
									borderRadius: '50%',
									backgroundColor: TYPE_COLORS[type] ?? '#6b7280',
									marginRight: '6px',
									verticalAlign: 'middle',
								}}
							/>
							{type}
						</li>
					))}
				</ul>
			</div>
			<div data-testid="legend-edges">
				<h4>Edge Types</h4>
				<ul>
					{EDGE_STYLES.map((style) => (
						<li key={style.label} data-testid="legend-edge">
							<svg
								width="24"
								height="8"
								style={{ verticalAlign: 'middle', marginRight: '6px' }}
								role="img"
								aria-label={`${style.label} edge`}
							>
								<title>{style.label} edge</title>
								<line
									x1="0"
									y1="4"
									x2="24"
									y2="4"
									stroke="#9ca3af"
									strokeWidth="2"
									strokeDasharray={style.dashed ? '4 4' : 'none'}
								/>
							</svg>
							<span data-testid="legend-edge-label">{style.label}</span>
						</li>
					))}
				</ul>
			</div>
		</div>
	)
}
