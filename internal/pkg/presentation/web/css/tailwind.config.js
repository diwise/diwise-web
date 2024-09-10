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
          'err-prim-surf': '#FBC1C1',
          'err-prim-surf-hover': '#FCD4D4',
          'gray-30': '#1C1C284D',
          'green-700': '#00733B',
          'input-surface': '#1C1C2880',
          'input-surface-dark': '#1C1C2880',
          'orange-600': '#DB6900',
          'primary-dark': '#1F1F25',
          'primary-green': '#00592D',
          'primary-green-accent': '#C9E4D7',
          'primary-surface': '#1C1C28F2',
          'primary-surface-hover': '#1C1C28E0',
          'primary-surface-dark': '#FFFFFFF2',
          'primary-surface-dark-hover': '#FFFFFFE0',
          'primary-surface-blue': '#005595',
          'primary-surface-white': '#FFFFFF33',
          'red-600': '#D62E2E',
          'secondary': '#E5E5E5',
          'secondary-dark': '#444450',
          'secondary-text': '#971A1A',
          'secondary-surface-hover': '#FFFFFF1A',
          'secondary-outline-hover': '#1C1C28A3',
          'secondary-outline-hover-dark': '#FFFFFFA3',
          'tertiary-surface': '#1C1C281F',
          'tertiary-surface-hover': '#1C1C2829',
          'white': '#FFFFFF',
          'white-30': '#FFFFFF4D',
          'white-50': '#FFFFFF80',
        },
        fontFamily: {
          heading: ['Raleway'],
          sans: ['Arial', 'sans-serif'],
        }  
      },
    },
    plugins: [],
    safelist: [
      'stroke-zinc-100',
      'stroke-secondary-text',
      'dark:stroke-zinc-100',
      'dark:border-white-50',
    ]
  }
  