/** @type {import('tailwindcss').Config} */

module.exports = {
    content: ["./internal/pkg/presentation/web/components/**/*.templ"],
    darkMode: 'class',
    theme: {
      extend: {
        colors: {
          'background-100': '#FAFAFA',
          'background-200': '#F0F0F0',
          'background-orange': '#AA4E1F',
          'content-background': '#2F2F3C',
          'dark-primary': '#1F1F25',
          'dark-secondary': '#444450',
          'darkmode-input-surface': '#1C1C2880',
          'divider-gray': '#1C1C284D',
          'divider-white': '#FFFFFF4D',
          'err-prim-surf': '#fbc1c1',
          'green-700': '#00733B',
          'input-outline': '#FFFFFF80',
          'input-surface': '#1c1c2880',
          'orange-600': '#DB6900',
          'primary-green': '#00592D',
          'primary-green-accent': '#C9E4D7',
          'primary-surface': '#1C1C28F2',
          'primary-surface-dark': '#FFFFFFF2',
          'primary-surface-white': '#FFFFFF33',
          'red-600': '#D62E2E',
          'secondary': '#E5E5E5',
          'secondary-text': '#971A1A',
          'tertiary-surface': '#1C1C281F',
          'tertiary-surface-hover': '#1C1C2829',
        },
        fontFamily: {
          heading: ['Raleway'],
          sans: ['Arial', 'sans-serif'],
        }  
      },
    },
    plugins: [],
  }
  