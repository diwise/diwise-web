/** @type {import('tailwindcss').Config} */

module.exports = {
    content: ["./internal/pkg/presentation/web/components/**/*.templ"],
    darkMode: 'class',
    theme: {
      extend: {
        colors: {
          'err-prim-surf': '#fbc1c1',
        },
        fontFamily: {
          heading: ['Raleway'],
          sans: ['Arial', 'sans-serif'],
        }  
      },
    },
    plugins: [],
  }
  