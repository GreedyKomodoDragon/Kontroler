import { Component } from 'solid-js';

interface LoadingIconProps {
  size?: number;  // Optional size prop to control the spinner's size
}

const LoadingIcon: Component<LoadingIconProps> = (props) => {
  const size = props.size || 8; // Default size is 8 (equivalent to 2rem or 32px in Tailwind)

  return (
    <div
      class={`w-${size} h-${size} border-4 border-gray-300 border-t-4 border-t-black rounded-full animate-spin`}
      style={{
        width: `${size}rem`,
        height: `${size}rem`,
      }}
    ></div>
  );
};

export default LoadingIcon;
