FROM mhart/alpine-node:16
# Create app directory
WORKDIR /workspace

# Install app dependencies
# A wildcard is used to ensure both package.json AND package-lock.json are copied
# where available (npm@5+)
COPY package*.json ./
COPY yarn.lock ./
COPY hardhat.config.ts ./
COPY tsconfig.json ./

RUN HUSKY_SKIP_INSTALL=1 npm install

# Build the Gravity bridge Solidity contract, run this after making any changes
RUN npm run typechain

EXPOSE 8545