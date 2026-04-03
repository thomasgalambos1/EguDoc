/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    "./src/**/*.{html,ts,scss}"
  ],
  theme: {
    extend: {}
  },
  plugins: [],
  corePlugins: {
    display: true,
    flexDirection: true,
    flexWrap: true,
    flex: true,
    flexGrow: true,
    flexShrink: true,
    alignItems: true,
    alignSelf: true,
    justifyContent: true,
    justifyItems: true,
    justifySelf: true,
    gap: true,
    padding: true,
    margin: true,
    width: true,
    height: true,
    minWidth: true,
    minHeight: true,
    maxWidth: true,
    maxHeight: true,
    overflow: true,
    position: true,
    inset: true,
    zIndex: true,
    grid: true,
    gridTemplateColumns: true,
    gridColumn: true,
    gridTemplateRows: true,
    gridRow: true,
    backgroundColor: false,
    textColor: false,
    borderColor: false,
    placeholderColor: false,
    ringColor: false,
    gradientColorStops: false,
  }
};
